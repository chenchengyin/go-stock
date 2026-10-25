"""Exploratory research, not a production signal or executable trading backtest.

Uses finalized daily close as an idealized late-day entry proxy. Candidate
definitions are fixed before examining the new run's period results. 2026 is a
chronological recheck, not genuinely untouched data (prior research used it).
"""
from pathlib import Path
import json
import sys
import numpy as np
import pandas as pd

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / "docs/research/2026-10-24-oversold-close"
TEMP = Path("/tmp/go-stock-oversold-20261024")
OUT.mkdir(parents=True, exist_ok=True)
FEE = 0.15  # percentage points; assumed round-trip friction, not a fee quote


def wilson(k, n):
    if not n:
        return [None, None]
    p, z = k / n, 1.959963984540054
    a = 1 + z*z/n
    c = (p + z*z/(2*n))/a
    h = z*np.sqrt(p*(1-p)/n + z*z/(4*n*n))/a
    return [100*(c-h), 100*(c+h)]


def stats(x):
    n = len(x)
    if not n:
        return {"n": 0, "days": 0}
    r = x["net"]
    daily = x.groupby("date").agg(net=("net", "mean"), yang=("yang", "mean"))
    return {
        "n": n, "days": len(daily), "stocks": int(x.code.nunique()),
        "yang_pct": 100*x.yang.mean(), "body_gt2_5_pct": 100*x.body_target.mean(),
        "gross_win_pct": 100*(x.ret > 1e-9).mean(), "net_win_pct": 100*(r > 1e-9).mean(),
        "close_gross_gt2_5_count": int((x.ret > 2.5+1e-9).sum()),
        "close_gross_gt2_5_pct": 100*(x.ret > 2.5+1e-9).mean(),
        "close_net_gt2_5_count": int((r > 2.5+1e-9).sum()),
        "close_net_gt2_5_pct": 100*(r > 2.5+1e-9).mean(),
        "mean_net_pct": r.mean(), "median_net_pct": r.median(),
        "q05_net_pct": r.quantile(.05), "worst_net_pct": r.min(),
        "loss_ge5_pct": 100*(r <= -5).mean(),
        "pf_net": r[r > 0].sum()/(-r[r < 0].sum()) if (r < 0).any() else None,
        "high_touch2_5_pct": 100*x.touch.mean(),
        "open_net_mean_pct": x.open_net.mean(), "open_net_win_pct": 100*(x.open_net > 0).mean(),
        "yang_but_net_loss_n": int((x.yang & (r <= 0)).sum()),
        "yang_ci95": wilson(int(x.yang.sum()), n), "net_win_ci95": wilson(int((r > 0).sum()), n),
        "equal_day_net_mean_pct": daily.net.mean(), "equal_day_yang_pct": 100*daily.yang.mean(),
        "largest_day_share_pct": 100*x.date.value_counts().max()/n,
        "next_locked_down_n": int(x.next_locked_down.sum()),
        "fee_sensitivity": {str(f): {"win_pct":100*(x.ret > f).mean(),"mean_pct":x.ret.mean()-f} for f in [.1,.2,.3]},
    }


def load_features():
    m = json.loads((TEMP / "metadata.json").read_text())
    d = pd.read_csv(TEMP / "daily.csv.gz", dtype={"code": str})
    d = d.sort_values(["code", "date"]).drop_duplicates(["code", "date"]).reset_index(drop=True)
    names = {s["Code"]: s["Name"] for s in m["Stocks"]}
    for pool in (m["SelectionPools"] or {}).values():
        names.update({s["Code"]:s["Name"] for s in pool})
    d["name"] = d.code.map(names).fillna("")
    dates = sorted(d.date.unique())
    date_number = {v:i for i,v in enumerate(dates)}
    d["di"] = d.date.map(date_number)
    g = d.groupby("code", sort=False)
    for lag in [1, 2, 6, 7, 8]:
        d[f"c{lag}"] = g.close.shift(lag)
    d["drop6"] = 100*(d.close/d.c6-1)  # existing seven-bar rule: SIX close intervals
    d["drop7true"] = 100*(d.close/d.c7-1)
    d["prev_drop6"] = 100*(d.c1/d.c7-1)
    d["prev_drop7true"] = 100*(d.c1/d.c8-1)
    d["gap"] = 100*(d.open/d.c1-1)
    d["dayret"] = 100*(d.close/d.c1-1)
    d["jump8"] = d.dayret.abs().gt(11).groupby(d.code).transform(lambda s:s.rolling(8,min_periods=1).max()).astype(bool)
    d["jump14"] = d.dayret.abs().gt(11).groupby(d.code).transform(lambda s:s.rolling(14,min_periods=1).max()).astype(bool)
    d["body"] = 100*(d.close/d.open-1)
    d["amp"] = 100*(d.high-d.low)/d.c1
    d["pos"] = (d.close-d.low)/(d.high-d.low)
    d["lower"] = 100*(d[["open","close"]].min(axis=1)-d.low)/d.c1
    for col in ["amp", "pos", "body", "dayret"]:
        d["prev_"+col] = d.groupby("code",sort=False)[col].shift(1)
    d["volume5"] = g.volume.transform(lambda s: s.shift(1).rolling(5,min_periods=5).mean())
    d["volratio"] = d.volume/d.volume5
    d["ma5"] = g.close.transform(lambda s:s.rolling(5,min_periods=5).mean())
    d["ma10"] = g.close.transform(lambda s:s.rolling(10,min_periods=10).mean())
    d["bias5"] = 100*(d.close/d.ma5-1)
    d["age"] = g.cumcount()+1
    d["contig8"] = (d.di-g.di.shift(7)==7)
    d["n_date"] = g.date.shift(-1)
    for c in ["open", "close", "high", "low"]:
        d["n_"+c] = g[c].shift(-1)
    d["n_di"] = g.di.shift(-1)
    d["next_contig"] = d.n_di-d.di==1
    d["ret"] = 100*(d.n_close/d.close-1)
    d["net"] = d.ret-FEE
    d["open_net"] = 100*(d.n_open/d.close-1)-FEE
    d["next_body"] = 100*(d.n_close/d.n_open-1)
    d["yang"] = d.next_body > 1e-9
    d["body_target"] = d.next_body > 2.5+1e-9
    # High touch is an upper-bound opportunity statistic, not executable P&L.
    d["touch"] = d.n_high >= np.ceil(d.close*1.025*100-1e-9)/100
    d["next_locked_down"] = (d.n_high-d.n_low).abs().lt(1e-7) & (d.ret < -9)
    valid = (d[["open","close","high","low"]]>0).all(axis=1)
    valid &= (d.high >= d[["open","close","low"]].max(axis=1)) & (d.low <= d[["open","close"]].min(axis=1))
    eligible = valid & d.contig8 & (d.age >= 20) & (d.volume>0)
    eligible &= d.code.str.match(r"^(60|00)") & ~d.name.str.contains(r"ST|退",case=False,regex=True)
    # Locked limit-up stocks cannot be assumed purchasable at the displayed close.
    eligible &= ~((d.high-d.low).abs().lt(1e-7) & (d.dayret > 9))
    # Outcome availability only; do not filter future returns or candle direction.
    outcome = d.next_contig & (d[["n_open","n_close","n_high","n_low"]]>0).all(axis=1)
    d["eligible"] = eligible
    # Available-universe breadth, not full-exchange breadth. Entry-day only.
    breadth = d[eligible].groupby("date").agg(breadth=("dayret",lambda s:(s>0).mean()),market_ret=("dayret","median"))
    d = d.join(breadth,on="date")
    candidates = eligible & ((d.drop6 <= -15) | (d.prev_drop6 <= -15))
    audit = {
        "rows":len(d),"symbols":int(d.code.nunique()),"first":d.date.min(),"last":d.date.max(),
        "eligible_rows":int(eligible.sum()),"oversold_rows_before_outcome":int(candidates.sum()),
        "missing_next":int((candidates & ~outcome).sum()),
        "rows_per_year":d.groupby(d.date.str[:4]).size().to_dict(),
        "last_days":d.groupby("date").size().tail(12).to_dict(),
        "schema_version":m["SchemaVersion"],"generated_at":m["GeneratedAt"],
        "market_cap_snapshot_at":m["MarketCapSnapshotAt"],"stock_count":len(m["Stocks"]),
        "dated_pool_dates":sorted(m["SelectionPools"] or {}),
        "amount_verified_pct":100*d.amount_verified.mean(),
        "kline_sources":pd.Series([s["KLineSource"] for s in m["Stocks"]]).value_counts().to_dict(),
    }
    # Include final days with no labels in audit only; never count them as losses.
    out = d[candidates & outcome].copy()
    out.to_csv(OUT/"features.csv.gz",index=False)
    (OUT/"data-audit.json").write_text(json.dumps(audit,ensure_ascii=False,indent=2))
    return out


def rules(d):
    # Three distinct entry hypotheses; original v3 is never silently relabeled.
    bases = {
        "A_original_v3_close": (d.prev_drop6<=-22.25+1e-9)&(d.prev_amp>=7.1-1e-9)&(d.prev_pos<=.8+1e-9)&(d.gap>=-4-1e-9)&(d.gap<-2-1e-9),
        "B_deepdrop_day_close": d.drop6<=-22.25+1e-9,
        "C_prevdeepdrop_any_gap_close": (d.prev_drop6<=-22.25+1e-9)&(d.prev_amp>=7.1-1e-9)&(d.prev_pos<=.8+1e-9),
    }
    filters = {
        "base":pd.Series(True,index=d.index),
        "bear":d.body<0,
        "small_bear":d.body.between(-3,0,inclusive="left"),
        "small_body":d.body.abs()<=2.5,
        "green":d.body>0,
        "recover":(d.pos>=.5)&(d.dayret>-5),
        "small_bear_not_low":d.body.between(-3,0,inclusive="left")&(d.pos>=.25),
        "small_bear_shrink":d.body.between(-3,0,inclusive="left")&(d.volratio<=1),
        "small_bear_liquid":d.body.between(-3,0,inclusive="left")&(d.amount_yi>=3)&d.amount_verified,
        "recover_shrink":(d.pos>=.5)&(d.dayret>-5)&(d.volratio<=1),
        "recover_liquid":(d.pos>=.5)&(d.dayret>-5)&(d.amount_yi>=3)&d.amount_verified,
        "not_panic":d.dayret>-5,
        "not_panic_shrink":(d.dayret>-5)&(d.volratio<=1),
        "not_panic_market":(d.dayret>-5)&(d.breadth>=.3),
        "lower_shadow":(d.lower>=1)&(d.pos>=.35)&(d.dayret>-7),
        "shrink":d.volratio<=1,
    }
    # Breadth filter is a research arm, not an inherited live trading rule.
    return {f"{a}__{b}":v & w for a,v in bases.items() for b,w in filters.items()}


def period_masks(d):
    return {
        "all":pd.Series(True,index=d.index),
        "2024":d.date<"2025-01-01",
        "2025":d.date.between("2025-01-01","2025-12-31"),
        "2026":d.date>="2026-01-01",
        "train_2024_2025H1":d.date<"2025-07-01",
        "validation_2025H2":d.date.between("2025-07-01","2025-12-31"),
        "recent_2026H2":d.date>="2026-07-01",
    }


def main():
    d = pd.read_csv(OUT/"features.csv.gz",dtype={"code":str}) if "--cached" in sys.argv else load_features()
    rr, pp = rules(d), period_masks(d)
    rows=[]
    for name, mask in rr.items():
        for period, pm in pp.items():
            s=stats(d[mask&pm]); rows.append({"rule":name,"period":period,**s})
    # Baseline sensitivity diagnostics, not a separate optimization search.
    for cutoff in [-15,-20,-22.25,-25,-30]:
        for period,pm in pp.items():
            rows.append({"rule":f"drop6_{cutoff:g}","period":period,**stats(d[(d.drop6<=cutoff)&pm])})
    (OUT/"results.json").write_text(json.dumps(rows,ensure_ascii=False,indent=2,default=lambda x:float(x)))
    flat=pd.DataFrame(rows).drop(columns=["fee_sensitivity","yang_ci95","net_win_ci95"])
    flat.to_csv(OUT/"results.csv",index=False)
    for period in ["all","train_2024_2025H1","validation_2025H2","2026"]:
        z=flat[(flat.period==period)&(flat.n>=20)].sort_values("net_win_pct",ascending=False)
        print("\n",period,"(descriptive ranking only)")
        print(z[["rule","n","days","yang_pct","net_win_pct","mean_net_pct","worst_net_pct"]].head(15).to_string(index=False))
    # Entire original-v3 sample retained, so manual label checks are possible.
    d[rr["A_original_v3_close__base"]].to_csv(OUT/"original-v3-close-samples.csv",index=False)
    print("\nSaved",OUT)


if __name__=="__main__":
    main()
