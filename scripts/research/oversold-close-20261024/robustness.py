"""Post-exploration diagnostics. No result here is an untouched holdout."""
import importlib.util
import json
from pathlib import Path
import numpy as np
import pandas as pd

spec=importlib.util.spec_from_file_location("analysis",Path(__file__).with_name("analyze.py"))
a=importlib.util.module_from_spec(spec)
spec.loader.exec_module(a)
OUT=a.OUT
d=pd.read_csv(OUT/"features.csv.gz",dtype={"code":str})


def cluster_ci(x, seed=20261024):
    """Resample signal dates, keeping every same-date stock together."""
    day=x.groupby("date").agg(n=("code","size"),yang=("yang","sum"),win=("net",lambda s:(s>0).sum()),net=("net","sum"))
    if not len(day): return {}
    arr=day.to_numpy();rng=np.random.default_rng(seed)
    ix=rng.integers(0,len(day),size=(10000,len(day)))
    pooled=arr[ix].sum(axis=1)
    pooled=pooled[:,1:]/pooled[:,0,None]
    equal=arr[:,1:]/arr[:,0,None]
    eqboot=equal[ix].mean(axis=1)
    scale=np.array([100,100,1])
    return {"date_cluster_pooled_ci95":(np.quantile(pooled,[.025,.975],axis=0)*scale).tolist(),
            "equal_day_ci95":(np.quantile(eqboot,[.025,.975],axis=0)*scale).tolist(),
            "columns":["yang_pct","net_win_pct","mean_net_pct"],
            "note":"Date clusters do not eliminate serial correlation or selection bias."}


def examine(x, name, period):
    ss=a.stats(x)
    if not len(x):return {"rule":name,"period":period,**ss}
    day=x.groupby("date").agg(n=("code","size"),yang=("yang","mean"),win=("net",lambda s:(s>0).mean()),net=("net","mean"),breadth=("breadth","first"))
    ss["portfolio_positive_day_pct"]=100*(day.net>0).mean()
    ss["equal_day_net_win_pct"]=100*day.win.mean()
    ss.update(cluster_ci(x))
    ss["remove_top_signal_dates"]={str(k):a.stats(x[~x.date.isin(day.nlargest(k,"n").index)]) for k in [1,3]}
    ss["one_stock_per_day"]={}
    for field,asc in [("amount_yi",False),("prev_drop6",True),("body",True)]:
        one=x.sort_values([field,"code"],ascending=[asc,True]).groupby("date").head(1)
        ss["one_stock_per_day"][field]=a.stats(one)
    day.sort_values("n",ascending=False).to_csv(OUT/f"{name}-{period}-days.csv")
    return {"rule":name,"period":period,**ss}


rr={k:v&~d.jump8 for k,v in a.rules(d).items()}
base=(d.prev_drop6<=-22.25+1e-9)&(d.prev_amp>=7.1-1e-9)&(d.prev_pos<=.8+1e-9)&~d.jump8
for t in [0,-2,-3,-4,-5,-6]:
    rr[f"D_body_lt_{t:g}"]=base&(d.body<t-1e-9)
main=rr["D_body_lt_-4"]
rr["D_minus4_no_near_limitdown"]=main&(d.dayret>-9.5)
rr["D_minus4_amount_ge3"]=main&(d.amount_yi>=3)&d.amount_verified
rr["D_minus4_amount_ge5"]=main&(d.amount_yi>=5)&d.amount_verified
rr["D_minus4_true7intervals"]=(d.prev_drop7true<=-22.25+1e-9)&(d.prev_amp>=7.1-1e-9)&(d.prev_pos<=.8+1e-9)&~d.jump8&(d.body<-4-1e-9)
rr["D_minus4_14bar_quarantine"]=main&~d.jump14
date_counts=d[main].groupby("date").size()
rr["D_minus4_batch_ge20"]=main&d.date.map(date_counts).ge(20)
rr["D_minus4_breadth_ge30"]=main&(d.breadth>=.3)
rr["D_minus4_breadth_ge50"]=main&(d.breadth>=.5)
rows=[]
periods=a.period_masks(d)
for name,mask in rr.items():
    for period,pm in periods.items():
        x=d[mask&pm]
        # Resampling and detailed sensitivity only for the candidates discussed.
        detail=name in ["D_body_lt_-4","D_body_lt_-6"]
        rows.append(examine(x,name,period) if detail else {"rule":name,"period":period,**a.stats(x)})
(OUT/"robustness.json").write_text(json.dumps(rows,ensure_ascii=False,indent=2,default=lambda x:float(x)))
cols=["rule","period","n","days","yang_pct","net_win_pct","close_gross_gt2_5_count","close_gross_gt2_5_pct","close_net_gt2_5_count","close_net_gt2_5_pct","mean_net_pct","equal_day_yang_pct","equal_day_net_mean_pct","largest_day_share_pct"]
pd.DataFrame(rows)[cols].to_csv(OUT/"robustness.csv",index=False)
samples=d[main].copy();samples.to_csv(OUT/"candidate-minus4-samples.csv",index=False)
print(pd.DataFrame(rows).query("rule.str.startswith('D_') and period in ['all','2026']",engine="python")[cols].to_string(index=False))

# Verify actual cached minute-bar coverage for the full candidate set.
coverage=[]
for p in (a.ROOT/"backend/data/cache/t0/intraday").glob("*/*.json"):
    s=json.loads(p.read_text()).get("series",{})
    code=s.get("code","").split(".")[0];date=s.get("date","")
    coverage.append({"code":code,"date":date,"bars":len(s.get("bars",[])),"candidate":bool(((samples.code==code)&(samples.date==date)).any())})
(OUT/"minute-coverage.json").write_text(json.dumps(coverage,ensure_ascii=False,indent=2))

# Label identities and exact disjoint example: a green next candle can lose money.
assert ((samples.n_close/samples.close-1)*100-samples.net-a.FEE).abs().max()<1e-8
assert (samples.yang==samples.n_close.gt(samples.n_open)).all()
assert samples[['code','date']].duplicated().sum()==0
assert not samples.jump8.any()
bad=samples[samples.yang & (samples.net<0)].sort_values("net")
bad.head(10).to_csv(OUT/"green-candle-but-loss-examples.csv",index=False)
print("Verified return identities, labels, uniqueness, entry-data quarantine; minute coverage",sum(x['candidate'] for x in coverage),"/",len(samples))
