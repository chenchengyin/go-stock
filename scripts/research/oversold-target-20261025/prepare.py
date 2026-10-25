"""Prepare entry-known features without modifying earlier research artifacts."""
from pathlib import Path
import hashlib
import json
import numpy as np
import pandas as pd

ROOT=Path(__file__).resolve().parents[3]
OUT=ROOT/'docs/research/2026-10-25-oversold-target-optimization'
RAW=Path('/tmp/go-stock-oversold-20261024/daily.csv.gz')
META=ROOT/'docs/research/2026-10-24-oversold-close/source-metadata.json'
OUT.mkdir(parents=True,exist_ok=True)


def main():
    d=pd.read_csv(RAW,dtype={'code':str}).sort_values(['code','date']).drop_duplicates(['code','date']).reset_index(drop=True)
    m=json.loads(META.read_text());names={s['Code']:s['Name'] for s in m['Stocks']}
    for pool in (m.get('SelectionPools') or {}).values():names.update({s['Code']:s['Name'] for s in pool})
    d['name']=d.code.map(names).fillna('');dates=sorted(d.date.unique())
    d['di']=d.date.map({v:i for i,v in enumerate(dates)});g=d.groupby('code',sort=False)
    for n in [1,2,5,6,7,8,20]:d[f'c{n}']=g.close.shift(n)
    d['age']=g.cumcount()+1
    d['ret1']=100*(d.close/d.c1-1);d['ret7']=100*(d.close/d.c7-1)
    d['prev_ret7']=100*(d.c1/d.c8-1)
    d['drop_old6']=100*(d.close/d.c6-1);d['prev_drop_old6']=100*(d.c1/d.c7-1)
    d['gap']=100*(d.open/d.c1-1);d['body']=100*(d.close/d.open-1)
    d['amp']=100*(d.high-d.low)/d.c1
    d['pos']=(d.close-d.low)/(d.high-d.low)
    d['lower_pct']=100*(d[['open','close']].min(axis=1)-d.low)/d.c1
    d['wick_fraction']=(d[['open','close']].min(axis=1)-d.low)/(d.high-d.low)
    for col in ['ret1','body','amp','pos','low']:
        d['prev_'+col]=d.groupby('code',sort=False)[col].shift(1)
    for n in [5,10,20]:
        d[f'ma{n}']=g.close.transform(lambda s:s.rolling(n,min_periods=n).mean())
        d[f'bias{n}']=100*(d.close/d[f'ma{n}']-1)
    d['ma5_rising']=d.ma5>=d.groupby('code',sort=False).ma5.shift(1)
    d['hold_low']=d.low>=d.prev_low;d['close_above_prev_low']=d.close>=d.prev_low
    d['before_drop_ret']=100*(d.c7/d.c20-1)
    delta=d.close-d.c1
    up=delta.clip(lower=0).groupby(d.code).transform(lambda s:s.rolling(6,min_periods=6).mean())
    down=(-delta.clip(upper=0)).groupby(d.code).transform(lambda s:s.rolling(6,min_periods=6).mean())
    d['rsi6']=100*up/(up+down)
    d['down7']=d.ret1.lt(0).groupby(d.code).transform(lambda s:s.rolling(7,min_periods=7).sum())
    d['limit_memory20']=d.ret1.ge(9.8).groupby(d.code).transform(lambda s:s.shift(1).rolling(20,min_periods=20).max()).fillna(0).astype(bool)
    d['volratio']=d.volume/g.volume.transform(lambda s:s.shift(1).rolling(5,min_periods=5).mean())
    units_ok=pd.Series(True,index=d.index)
    for n in range(1,6):units_ok&=d.volume_unit.eq(g.volume_unit.shift(n))
    d.loc[~units_ok,'volratio']=np.nan
    for n in [8,21]:
        d[f'jump{n}']=d.ret1.abs().gt(11).groupby(d.code).transform(lambda s:s.rolling(n,min_periods=1).max()).astype(bool)
    d['contig21']=d.di-g.di.shift(20)==20
    d['n_date']=g.date.shift(-1);d['n_di']=g.di.shift(-1)
    for col in ['open','close','high','low']:d['n_'+col]=g[col].shift(-1)
    d['gross']=100*(d.n_close/d.close-1);d['net']=d.gross-.15
    d['next_anomaly']=d.gross.abs()>11
    d['target_raw']=d.net>2.5+1e-9
    d['target']=d.target_raw&~d.next_anomaly
    d['next_locked_down']=(d.n_high-d.n_low).abs().lt(1e-7)&(d.gross<-9)
    valid=(d[['open','close','high','low']]>0).all(axis=1)&(d.high>=d[['open','close','low']].max(axis=1))&(d.low<=d[['open','close']].min(axis=1))
    eligible=valid&d.contig21&(d.age>=21)&(d.volume>0)&d.amount_verified
    eligible&=d.code.str.match(r'^(00|60)')&~d.name.str.contains(r'ST|退',case=False)
    eligible&=~((d.high-d.low).abs().lt(1e-7)&(d.ret1>9))
    # Breadth uses entry-known data from the available universe; no outcome filter.
    market=d[eligible].groupby('date').agg(breadth=('ret1',lambda s:(s>0).mean()),market_ret=('ret1','median'),panic_fraction=('ret1',lambda s:(s<=-5).mean()))
    market['breadth_delta']=market.breadth.diff()
    d=d.join(market,on='date');d['relative_ret']=d.ret1-d.market_ret
    outcome=(d.n_di-d.di==1)&(d[['n_open','n_close','n_high','n_low']]>0).all(axis=1)
    # No next-year outcome in the preceding year's fold.
    d['same_year_outcome']=d.date.str[:4]==d.n_date.str[:4]
    pool=eligible&~d.jump21&((d.ret7<=-10+1e-9)|(d.prev_ret7<=-10+1e-9))
    audit={'raw_rows':len(d),'raw_symbols':d.code.nunique(),'dates':len(dates),'last':dates[-1],
           'eligible_rows':int(eligible.sum()),'oversold_before_outcome':int(pool.sum()),
           'missing_next':int((pool&~outcome).sum()),'next_anomaly_n':int((pool&outcome&d.next_anomaly).sum()),
           'raw_sha256':hashlib.sha256(RAW.read_bytes()).hexdigest(),
           'generated_at':m['GeneratedAt'],'target':'net > 2.5 percentage points; cost 0.15 pp',
           'next_anomaly_handling':'Keep in denominator, do not claim confirmed success',
           'data_limitations':['current snapshot universe','current ST names, not full historical states','unadjusted daily prices','final close as entry proxy']}
    # A date-level available-universe outcome control, never an input feature.
    control=d[eligible&~d.jump21&outcome].groupby('date').agg(universe_n=('code','size'),universe_target=('target','mean'),universe_net=('net','mean'))
    control.to_csv(OUT/'same-date-control.csv')
    d=d[pool&outcome].sort_values(['date','amount_yi','code'],ascending=[True,False,True]).reset_index(drop=True)
    d.to_csv(OUT/'features.csv.gz',index=False)
    audit['research_rows']=len(d);audit['research_symbols']=d.code.nunique()
    audit['rows_by_year']=d.groupby(d.date.str[:4]).size().to_dict()
    (OUT/'data-audit.json').write_text(json.dumps(audit,ensure_ascii=False,indent=2))
    print(json.dumps(audit,ensure_ascii=False,indent=2))


if __name__=='__main__':main()
