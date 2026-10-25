"""Entry-price audit of the already frozen F1 daily-selected cohort.

This is NOT a complete 14:50 signal backtest: selection used final daily data,
and minute-price sampling cannot reconstruct all intraminute extremes.
No condition parameters or candidate rankings are optimized here.
"""
from concurrent.futures import ThreadPoolExecutor,as_completed
from datetime import datetime,timedelta,timezone
from pathlib import Path
import json
import subprocess
import numpy as np
import pandas as pd

ROOT=Path(__file__).resolve().parents[3]
OUT=ROOT/'docs/research/2026-10-25-oversold-target-optimization'
CACHE=OUT/'minute-cache';CACHE.mkdir(exist_ok=True)
TZ=timezone(timedelta(hours=8))
HEADERS={'Content-Type':'application/json','Origin':'https://stockpage.10jqka.com.cn',
    'Referer':'https://stockpage.10jqka.com.cn/','User-Agent':'Mozilla/5.0',
    'X-Auth-ProgId':'7047','X-Auth-Type':'ths','X-Auth-Version':'1.0',
    'X-Auth-AppName':'AINVEST','Source-Id':'hxkline-NEWS_appNewsFlowHome_Page','Platform':'hxkline'}


def fetch(row):
    code,date=row['code'],row['date'];dest=CACHE/f'{date}-{code}.json'
    status={'code':code,'date':date,'daily_close':row['close']}
    if not dest.exists():
        body={'code_list':[{'codes':[code],'market':'17' if code.startswith('6') else '33'}],
              'trade_class':'intraday','trade_date':int(date.replace('-','')),'gpid':1}
        args=['curl','-sS','--max-time','15','--retry','1','--retry-delay','1','-o',str(dest),'-w','%{http_code}']
        for k,v in HEADERS.items():args.extend(['-H',f'{k}: {v}'])
        args.extend(['--data-binary',json.dumps(body),'https://quota-h.10jqka.com.cn/fuyao/common_hq_aggr/quote/v1/single_trend'])
        run=subprocess.run(args,capture_output=True,text=True)
        if run.returncode or run.stdout!='200':
            if dest.exists():dest.unlink()
            return {**status,'status':'request_failed','detail':run.stderr[:180],'http':run.stdout}
    try:
        obj=json.loads(dest.read_text());assert obj.get('status_code')==0
        qs=[q for q in obj.get('data',{}).get('quote_data',[]) if str(q.get('code'))==code]
        assert len(qs)==1
        q=qs[0];ix={str(v):i for i,v in enumerate(q['data_fields'])};points=[]
        for values in q['value']:
            at=datetime.fromtimestamp(float(values[ix['1']])/1000,TZ)
            assert at.date().isoformat()==date
            clock=at.strftime('%H:%M');price=float(values[ix['10']]);assert price>0
            points.append((clock,price,float(values[ix['13']]),float(values[ix['19']])))
        points.sort();assert len(points)>=230
        assert len({p[0] for p in points})==len(points)
        assert all(points[i][2]>=points[i-1][2] and points[i][3]>=points[i-1][3] for i in range(1,len(points)))
        bytime={p[0]:p for p in points};assert all(t in bytime for t in ['14:50','14:55','15:00'])
        assert abs(bytime['15:00'][1]-row['close'])<=.011
        status['status']='ok';status['points']=len(points)
        for t in ['14:50','14:55']:
            point=bytime[t];tail=[p[1] for p in points if p[0]<=t]+[row['open']]
            status[t+'_price']=point[1];status[t+'_cum_volume_hands']=point[2]/100
            status[t+'_cum_amount_yi']=point[3]/1e8
            status[t+'_sampled_amp']=100*(max(tail)-min(tail))/row['c1']
        return status
    except Exception as e:return {**status,'status':'invalid_series','detail':str(e)}


def main():
    d=pd.read_csv(OUT/'F1-selected-samples.csv',dtype={'code':str});records=d.to_dict('records');out=[]
    with ThreadPoolExecutor(max_workers=2) as pool:
        futures=[pool.submit(fetch,row) for row in records]
        for future in as_completed(futures):
            out.append(future.result())
            if len(out)%50==0:print('Minute audit',len(out),'/',len(records),'ok',sum(x['status']=='ok' for x in out),flush=True)
    minute=pd.DataFrame(out).sort_values(['date','code']);minute.to_csv(OUT/'F1-minute-coverage.csv',index=False)
    x=d.merge(minute,on=['code','date'],validate='one_to_one')
    # Prior five close-to-close changes are already known before entry day.
    raw=pd.read_csv('/tmp/go-stock-oversold-20261024/daily.csv.gz',dtype={'code':str}).sort_values(['code','date'])
    change=raw.close-raw.groupby('code').close.shift(1)
    raw['prior_up5']=change.clip(lower=0).groupby(raw.code).transform(lambda v:v.shift(1).rolling(5,min_periods=5).sum())
    raw['prior_down5']=(-change.clip(upper=0)).groupby(raw.code).transform(lambda v:v.shift(1).rolling(5,min_periods=5).sum())
    x=x.merge(raw[['code','date','prior_up5','prior_down5']],on=['code','date'],validate='one_to_one')
    results=[]
    for label,pm in [('all',x.date.ne('')),('2024',x.date.str.startswith('2024')),('2025',x.date.str.startswith('2025')),('2026',x.date.str.startswith('2026'))]:
        cohort=x[pm];valid=cohort[cohort.status.eq('ok')].copy()
        for t in ['14:50','14:55']:
            net=100*(valid.n_close/valid[t+'_price']-1)-.15
            valid[t+'_net']=net
            valid[t+'_target']=(net>2.5+1e-9)&~valid.next_anomaly
            delta=valid[t+'_price']-valid.c1
            rsi=100*(valid.prior_up5+delta.clip(lower=0))/(valid.prior_up5+valid.prior_down5+delta.abs())
            vr=valid[t+'_cum_volume_hands']/(valid.volume/valid.volratio)
            comparable=valid.volume_unit.eq('hands')
            other=(rsi<=20+1e-9)&(vr>=.7-1e-9)&(vr<1.5-1e-9)
            pass_lower_bound=other&(valid[t+'_sampled_amp']>=7-1e-9)&comparable
            definitely_fail=(~other)&comparable
            result={'period':label,'entry_time':t,'cohort_n':len(cohort),'valid_n':len(valid),'missing_n':len(cohort)-len(valid),
                'days':valid.date.nunique(),'target_n':int(valid[t+'_target'].sum()),
                'stock_target_pct':float(100*valid[t+'_target'].mean()),
                'day_target_pct':float(100*valid.groupby('date')[t+'_target'].mean().mean()),
                'day_mean_net_pct':float(valid.groupby('date')[t+'_net'].mean().mean()),
                'same_cohort_close_target_pct':float(100*valid.target.mean()),
                'confirmed_sampled_conditions_pass_n':int(pass_lower_bound.sum()),
                'known_conditions_fail_n':int(definitely_fail.sum()),
                'amp_or_volume_units_indeterminate_n':int((~pass_lower_bound&~definitely_fail).sum()),
                'scope':'Fixed final-daily-selected F1 cohort only. Not an executable point-in-time signal backtest.'}
            results.append(result)
    x.to_csv(OUT/'F1-minute-entry-audit.csv',index=False)
    (OUT/'F1-minute-entry-summary.json').write_text(json.dumps(results,ensure_ascii=False,indent=2))
    print(pd.DataFrame(results).drop(columns='scope').to_string(index=False),flush=True)
    print('No frozen conditions changed. No price/volume data from after entry used in the intraday recomputation; original cohort selection still used final daily data.')


if __name__=='__main__':main()
