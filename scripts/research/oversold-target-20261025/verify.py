"""Independent arithmetic, selection and snapshot checks; paired-date diagnostics."""
from pathlib import Path
import hashlib
import importlib.util
import json
import numpy as np
import pandas as pd

HERE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('search',HERE/'optimize.py');s=importlib.util.module_from_spec(spec);spec.loader.exec_module(s)
spec2=importlib.util.spec_from_file_location('audit',HERE/'evaluate.py');e=importlib.util.module_from_spec(spec2);spec2.loader.exec_module(e)
OUT=s.OUT


def main():
    r=s.Research();d=r.d
    frozen=json.loads((OUT/'frozen-candidates.json').read_text());manifest=json.loads((OUT/'audit-manifest.json').read_text())
    assert hashlib.sha256((OUT/'frozen-candidates.json').read_bytes()).hexdigest()==manifest['frozen_candidates_sha256']
    assert hashlib.sha256((OUT/'training-shortlist.json').read_bytes()).hexdigest()==manifest['shortlist_sha256']
    assert hashlib.sha256((OUT/'features.csv.gz').read_bytes()).hexdigest()==manifest['features_sha256']
    assert (d.n_di-d.di==1).all() and not d.jump21.any()
    assert np.max(np.abs(100*(d.n_close/d.close-1)-.15-d.net))<1e-8
    assert np.array_equal(d.target.to_numpy(),((100*(d.n_close/d.close-1)-.15>2.5+1e-9)&~d.next_anomaly).to_numpy())
    assert not (100*(102.65/100-1)-.15>2.5+1e-9)
    assert 100*(102.66/100-1)-.15>2.5+1e-9
    complete=r.folds['2024']|r.folds['2025']|r.folds['2026']
    paired={};rng=np.random.default_rng(20261025)
    for row in frozen['candidates']:
        name=row['id'];mask=r.mask(row['parts'])&complete
        # Independent pandas implementation, not a call to Research.selected.
        expected=d[mask].sort_values(['date','amount_yi','code'],ascending=[True,False,True]).groupby('date',sort=False).head(5)
        saved=pd.read_csv(OUT/f'{name}-selected-samples.csv',dtype={'code':str})
        assert list(zip(expected.code,expected.date))==list(zip(saved.code,saved.date))
        assert saved.groupby('date').size().max()<=5
        cd=d[e.cooldown(r,mask)]
        assert cd.groupby('code').di.diff().dropna().ge(5).all()
        rootmask=r.masks[row['parts'][0]]&complete
        root=d.iloc[r.selected(rootmask)]
        for period,pm in [('all',saved.date.ne('')),('2026',saved.date.str.startswith('2026'))]:
            z=saved[pm];f=z.groupby(['date','di']).target.mean();b=root.groupby(['date','di']).target.mean()
            pair=f.to_frame('candidate').join(b.rename('root'),how='left');assert not pair.isna().any().any()
            diff=(pair.candidate-pair.root).to_numpy();di=pair.index.get_level_values('di').to_numpy();lo=di.min();n=di.max()-lo+1
            values=np.zeros((n,2));values[di-lo,0]=diff;values[di-lo,1]=1
            block=min(5,n);nb=(n+block-1)//block
            starts=rng.integers(0,n-block+1,size=(5000,nb));ix=(starts[:,:,None]+np.arange(block)).reshape(5000,-1)[:,:n]
            totals=values[ix].sum(axis=1);totals=totals[totals[:,1]>0]
            delta=100*totals[:,0]/totals[:,1]
            paired[name+'_'+period]={'dates':len(pair),'equal_day_target_lift_pp':float(100*diff.mean()),
                'moving_5_session_block_ci95_pp':np.quantile(delta,[.025,.975]).tolist(),
                'note':'Matched signal dates, root top5 vs candidate top5; exploratory interval, no multiple-testing correction.'}
    f1=pd.read_csv(OUT/'F1-selected-samples.csv',dtype={'code':str})
    assert len(f1)==818 and f1.date.nunique()==280 and f1.target.sum()==255
    minute=pd.read_csv(OUT/'F1-minute-entry-audit.csv',dtype={'code':str})
    assert len(minute)==818 and minute.status.eq('ok').all()
    for t,expected in [('14:50',246),('14:55',251)]:
        net=100*(minute.n_close/minute[t+'_price']-1)-.15
        assert int(((net>2.5+1e-9)&~minute.next_anomaly).sum())==expected
    volume_checked=0;volume_bad=[]
    for row in f1.to_dict('records'):
        if row['volume_unit']!='hands':continue
        q=json.loads((OUT/'minute-cache'/f"{row['date']}-{row['code']}.json").read_text())['data']['quote_data'][0]
        ix={v:i for i,v in enumerate(q['data_fields'])};cum=float(q['value'][-1][ix['13']])/100
        volume_checked+=1
        if abs(cum/row['volume']-1)>.001:volume_bad.append((row['date'],row['code']))
    assert not volume_bad,volume_bad
    (OUT/'paired-date-improvement.json').write_text(json.dumps(paired,ensure_ascii=False,indent=2))
    result={'checks':'passed','frozen_candidates':len(frozen['candidates']),'frozen_hash_unchanged':True,
        'strict_threshold_boundary_checked':True,'top5_independently_reproduced':True,'cooldown_spacing_checked':True,
        'F1_selected_n':818,'F1_dates':280,'F1_close_targets':255,'F1_1450_targets':246,'F1_1455_targets':251,
        'minute_series_valid':818,'minute_volume_unit_crosschecks':volume_checked}
    (OUT/'verification.json').write_text(json.dumps(result,ensure_ascii=False,indent=2))
    print(json.dumps(result,ensure_ascii=False,indent=2));print(json.dumps(paired,ensure_ascii=False,indent=2))


if __name__=='__main__':main()
