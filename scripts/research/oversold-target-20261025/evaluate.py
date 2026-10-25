"""Audit frozen candidates once. Does not change or select candidate conditions."""
from pathlib import Path
import hashlib
import importlib.util
import json
import numpy as np
import pandas as pd

spec=importlib.util.spec_from_file_location('search',Path(__file__).with_name('optimize.py'))
s=importlib.util.module_from_spec(spec);spec.loader.exec_module(s)
OUT=s.OUT


def cooldown(r,mask,cap=5,distance=5):
    ix=np.flatnonzero(mask);codes=r.d.code.to_numpy();di=r.d.di.to_numpy();last={};chosen=[]
    current=-1;used=0
    for i in ix:
        if r.day[i]!=current:current=r.day[i];used=0
        if used>=cap or di[i]-last.get(codes[i],-1000000)<distance:continue
        chosen.append(i);last[codes[i]]=di[i];used+=1
    out=np.zeros(len(r.d),bool);out[chosen]=True
    return out


def extended(r,mask,cap=5):
    m=r.metrics(mask,cap);ix=r.selected(mask,cap)
    if not len(ix):return m
    z=r.d.iloc[ix];day=z.groupby('date').agg(n=('code','size'),target=('target','mean'),net=('net','mean'))
    raw_counts=r.d[mask].date.value_counts().rename_axis('date').reset_index(name='n').sort_values(['n','date'],ascending=[False,True])
    topdates=raw_counts.head(3).date.tolist()
    no3=mask&~r.d.date.isin(topdates).to_numpy()
    known=mask&~r.d.date.isin(s.KNOWN_DATES).to_numpy()
    # Do not reselect different candidates after learning which outcomes failed.
    m.update({'stocks':int(z.code.nunique()),'target_raw_n':int(z.target_raw.sum()),
              'raw_target_pct':float(100*r.d[mask].target.mean()),
              'net_positive_pct':float(100*z.net.gt(0).mean()),
              'median_net_pct':float(z.net.median()),'q05_net_pct':float(z.net.quantile(.05)),
              'portfolio_positive_day_pct':float(100*day.net.gt(0).mean()),
              'portfolio_gt2_5_day_pct':float(100*day.net.gt(2.5+1e-9).mean()),
              'next_locked_down_n':int(z.next_locked_down.sum()),
              'top3_dates':topdates,'remove3':r.metrics(no3,cap),'known3_removed':r.metrics(known,cap)})
    m['cost_sensitivity']={str(f):{'target_n':int(((z.gross-f>2.5+1e-9)&~z.next_anomaly).sum()),
                                  'stock_target_pct':float(100*((z.gross-f>2.5+1e-9)&~z.next_anomaly).mean())} for f in [.1,.15,.2,.3,.5]}
    m['entry_slippage_sensitivity']={str(v):float(100*((100*(z.n_close/(z.close*(1+v/100))-1)-.15>2.5+1e-9)&~z.next_anomaly).mean()) for v in [0,.1,.2,.3]}
    return m


def bootstrap(r,mask,cap=5):
    ix=r.selected(mask,cap)
    if not len(ix):return {}
    z=r.d.iloc[ix];lo=int(z.di.min());hi=int(z.di.max());n=hi-lo+1
    count=np.bincount(z.di.to_numpy()-lo,minlength=n).astype(float)
    hit=np.bincount(z.di.to_numpy()-lo,weights=z.target.to_numpy(float),minlength=n)
    pnl=np.bincount(z.di.to_numpy()-lo,weights=z.net.to_numpy(float),minlength=n)
    use=count>0;dailyhit=np.divide(hit,count,out=np.zeros(n),where=use)
    dailynet=np.divide(pnl,count,out=np.zeros(n),where=use)
    arr=np.column_stack([count,hit,use.astype(float),dailyhit,dailynet])
    rng=np.random.default_rng(20261025)
    # Five consecutive MARKET-session blocks. No-signal dates remain in blocks.
    length=min(5,n);nb=(n+length-1)//length
    starts=rng.integers(0,n-length+1,size=(5000,nb))
    ix=(starts[:,:,None]+np.arange(length)).reshape(5000,-1)[:,:n]
    totals=arr[ix].sum(axis=1);valid=(totals[:,0]>0)&(totals[:,2]>0)
    totals=totals[valid]
    vals=np.column_stack([100*totals[:,1]/totals[:,0],100*totals[:,3]/totals[:,2],totals[:,4]/totals[:,2]])
    return {'method':'5000 moving-block resamples of 5 market sessions, no-signal sessions preserved',
            'columns':['stock_target_pct','equal_day_target_pct','equal_day_mean_net_pct'],
            'ci95':np.quantile(vals,[.025,.975],axis=0).tolist(),
            'limitation':'Descriptive; excludes neither historical selection bias nor multiple-testing bias.'}


def main():
    frozen_path=OUT/'frozen-candidates.json';frozen_bytes=frozen_path.read_bytes();frozen=json.loads(frozen_bytes)
    short=OUT/'training-shortlist.json'
    assert hashlib.sha256(short.read_bytes()).hexdigest()==frozen['shortlist_sha256']
    r=s.Research();d=r.d;complete=r.folds['2024']|r.folds['2025']|r.folds['2026']
    masks={k:r.masks[k] for k in ['now10','now15','now20','prev10','prev15','prev20']}
    d4=(d.prev_drop_old6<=-22.25+1e-9)&(d.prev_amp>=7.1-1e-9)&(d.prev_pos<=.8+1e-9)&(d.body<-4-1e-9)
    masks['old_D4_common_universe']=d4.to_numpy();masks['old_D6_common_universe']=(d4&(d.body<-6-1e-9)).to_numpy()
    for row in frozen['candidates']:masks[row['id']]=r.mask(row['parts'])
    pp={'all':complete,'2024':r.folds['2024'],'2025':r.folds['2025'],'2026':r.folds['2026'],
        '2026H1':r.folds['2026']&(d.date<'2026-07-01').to_numpy(),
        '2026H2':r.folds['2026']&(d.date>='2026-07-01').to_numpy()}
    results=[]
    for name,mask in masks.items():
        for period,pm in pp.items():results.append({'rule':name,'period':period,**extended(r,mask&pm)})
    detailed={};universe=pd.read_csv(OUT/'same-date-control.csv').set_index('date')
    for row in frozen['candidates']:
        name=row['id'];mask=masks[name]&complete;root=r.masks[row['parts'][0]]
        selected_ix=r.selected(mask);z=d.iloc[selected_ix].copy();z['rule_id']=name
        z.to_csv(OUT/f'{name}-selected-samples.csv',index=False)
        d[mask].to_csv(OUT/f'{name}-raw-candidates.csv.gz',index=False)
        daily=z.groupby('date').agg(n=('code','size'),target_n=('target','sum'),target_rate=('target','mean'),mean_net=('net','mean'))
        daily.to_csv(OUT/f'{name}-daily.csv')
        cd=cooldown(r,mask);detail={'parts':row['parts'],'conditions':[r.defs[k]['label'] for k in row['parts']],
            'root':row['parts'][0],'bootstrap':{},'cooldown5':{},'one_per_day':{},'matched_date_root':{},'matched_date_universe':{}}
        for period,pm in pp.items():
            detail['cooldown5'][period]=extended(r,cd&pm)
            detail['one_per_day'][period]=extended(r,mask&pm,cap=1)
            exactdates=d.iloc[r.selected(mask&pm)].date.unique()
            detail['matched_date_root'][period]=extended(r,root&pm&d.date.isin(exactdates).to_numpy())
            common=universe.reindex(exactdates).dropna()
            detail['matched_date_universe'][period]={'days':len(common),'day_target_pct':float(100*common.universe_target.mean()),'day_mean_net_pct':float(common.universe_net.mean())}
            if period in ['all','2026']:detail['bootstrap'][period]=bootstrap(r,mask&pm)
        allstat=next(z for z in results if z['rule']==name and z['period']=='all')
        recent=next(z for z in results if z['rule']==name and z['period']=='2026')
        detail['gates']={
            'overall_scale':allstat['n']>=500 and allstat['days']>=150,
            'recent_scale':recent['n']>=100 and recent['days']>=40,
            'selected_concentration':bool(allstat['selected_top1_share_pct']<10 and allstat['selected_top3_share_pct']<20),
            'positive_day_mean_each_year':all(next(z for z in results if z['rule']==name and z['period']==yr).get('day_mean_net_pct',-1)>0 for yr in ['2024','2025','2026']),
            'better_than_root_day_target_each_year':all(next(z for z in results if z['rule']==name and z['period']==yr).get('day_target_pct',0)>next(z for z in results if z['rule']==row['parts'][0] and z['period']==yr).get('day_target_pct',0) for yr in ['2024','2025','2026']),
        }
        detailed[name]=detail
    s.dump(OUT/'evaluation.json',results);s.dump(OUT/'candidate-diagnostics.json',detailed)
    cols=['rule','period','n','days','target_n','stock_target_pct','day_target_pct','remove3_day_target_pct','known3_removed_day_target_pct','net_positive_pct','mean_net_pct','day_mean_net_pct','loss_ge5_pct','worst_net_pct','raw_n','raw_target_pct','raw_top3_share_pct','selected_top3_share_pct','next_anomaly_n']
    pd.DataFrame(results)[cols].to_csv(OUT/'evaluation.csv',index=False)
    meta={'frozen_candidates_sha256':hashlib.sha256(frozen_bytes).hexdigest(),'shortlist_sha256':frozen['shortlist_sha256'],
          'features_sha256':hashlib.sha256((OUT/'features.csv.gz').read_bytes()).hexdigest(),
          'protocol_sha256':hashlib.sha256((OUT/'研究约定.md').read_bytes()).hexdigest(),
          'holdout_checked':True,'refit_after_2026_audit':False}
    s.dump(OUT/'audit-manifest.json',meta)
    assert frozen_path.read_bytes()==frozen_bytes
    display=pd.DataFrame(results)
    print(display[display.rule.str.startswith('F') & display.period.isin(['all','2024','2025','2026'])][cols].to_string(index=False))
    for name,x in detailed.items():print(name,'gates',x['gates'],'one/day2026',x['one_per_day']['2026'],'cooldown2026',x['cooldown5']['2026'])
    print('Frozen file unchanged; audit completed without refit.')


if __name__=='__main__':main()
