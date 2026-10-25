"""Bounded, chronological search. Run train, validate, audit in separate calls.

All phases remain exploratory because the historical data were previously seen.
No next-day field is permitted in any condition definition.
"""
from pathlib import Path
import hashlib
import json
import sys
import numpy as np
import pandas as pd

ROOT=Path(__file__).resolve().parents[3]
OUT=ROOT/'docs/research/2026-10-25-oversold-target-optimization'
KNOWN_DATES=['2024-02-05','2024-02-07','2026-07-20']


def definitions():
    defs={}
    def add(key,field,op,value,group,label):
        assert not field.startswith('n_') and field not in ['gross','net','target','target_raw','next_anomaly']
        defs[key]={'field':field,'op':op,'value':value,'group':group,'label':label}
    for fld,tag,cn in [('ret7','now','当日'),('prev_ret7','prev','前一日')]:
        for v in [10,15,20]:add(f'{tag}{v}',fld,'le',-v,'root',f'截至{cn}七个收益间隔累计跌幅≥{v}%')
    for v in [2,4,6]:add(f'body_lt_m{v}','body','lt',-v,'body',f'当日阴线实体>{v}%')
    for lo,hi in [(-6,-2),(-4,0),(-2,2),(0,2.5)]:add(f'body_{lo}_{hi}','body','range',[lo,hi],'body',f'当日实体[{lo}%,{hi}%)')
    add('body_green','body','gt',0,'body','当日收阳')
    for lo,hi in [(-9.5,-5),(-8,-2),(-5,0),(0,3)]:add(f'ret_{lo}_{hi}','ret1','range',[lo,hi],'ret1',f'当日涨跌幅[{lo}%,{hi}%)')
    add('not_limitdown','ret1','gt',-9.5,'ret1','当日收盘跌幅小于9.5%')
    for lo,hi in [(-5,-2),(-2,0),(0,2),(2,5)]:add(f'gap_{lo}_{hi}','gap','range',[lo,hi],'gap',f'当日开盘缺口[{lo}%,{hi}%)')
    add('gap_nonnegative','gap','ge',0,'gap','当日平开或高开')
    for v in [.2,.5]:add(f'pos_le{v}','pos','le',v,'pos',f'当日收盘位置≤{100*v:g}%')
    for v in [.5,.8]:add(f'pos_ge{v}','pos','ge',v,'pos',f'当日收盘位置≥{100*v:g}%')
    add('pos_middle','pos','range',[.2,.6],'pos','当日收盘位置[20%,60%)')
    for v in [.7,1]:add(f'volume_le{v}','volratio','le',v,'volume',f'当日量/前5日均量≤{v:g}')
    for lo,hi in [(.7,1.5),(1,2.5)]:add(f'volume_{lo}_{hi}','volratio','range',[lo,hi],'volume',f'当日量/前5日均量[{lo},{hi})')
    add('volume_ge1.5','volratio','ge',1.5,'volume','当日量/前5日均量≥1.5')
    for v in [1,3,5]:add(f'amount_ge{v}','amount_yi','ge',v,'amount',f'当日成交额≥{v}亿元')
    add('amount_le20','amount_yi','le',20,'amount','当日成交额≤20亿元')
    for v in [3,5,7]:add(f'amp_ge{v}','amp','ge',v,'amp',f'当日振幅≥{v}%')
    add('amp_le5','amp','le',5,'amp','当日振幅≤5%')
    for v in [1,2]:add(f'lower_ge{v}','lower_pct','ge',v,'lower',f'当日下影/昨收≥{v}%')
    add('wick_ge.3','wick_fraction','ge',.3,'lower','当日下影占振幅≥30%')
    for v in [2,4]:add(f'prev_body_lt_m{v}','prev_body','lt',-v,'prev_body',f'前一日阴线实体>{v}%')
    add('prev_ret_le_m3','prev_ret1','le',-3,'prev_ret','前一日收跌至少3%')
    add('prev_ret_ge0','prev_ret1','ge',0,'prev_ret','前一日收盘不跌')
    for v in [5,7]:add(f'prev_amp_ge{v}','prev_amp','ge',v,'prev_amp',f'前一日振幅≥{v}%')
    add('prev_pos_le.8','prev_pos','le',.8,'prev_pos','前一日收盘位置≤80%')
    for v in [20,30]:add(f'rsi_le{v}','rsi6','le',v,'rsi',f'简单6日RSI≤{v}')
    for v in [3,5,8]:add(f'bias5_le_m{v}','bias5','le',-v,'bias5',f'5日乖离≤−{v}%')
    add('bias20_le_m10','bias20','le',-10,'bias20','20日乖离≤−10%')
    add('bias20_ge_m10','bias20','ge',-10,'bias20','20日乖离≥−10%')
    for v in [5,6]:add(f'down7_ge{v}','down7','ge',v,'down7',f'近7日收跌天数≥{v}')
    for field,label in [('hold_low','当日最低不低于前日最低'),('close_above_prev_low','当日收盘不低于前日最低'),('ma5_rising','当日5日均线不下降'),('limit_memory20','此前20日有收盘涨停记忆')]:
        add(field,field,'bool',True,field,label)
    add('before_drop_ge10','before_drop_ret','ge',10,'before_drop','下跌前13个收益间隔涨幅≥10%')
    for v in [.3,.5,.7]:add(f'breadth_ge{v}','breadth','ge',v,'breadth',f'当日可用样本池上涨占比≥{100*v:g}%')
    add('breadth_le.3','breadth','le',.3,'breadth','当日可用样本池上涨占比≤30%')
    add('breadth_improving','breadth_delta','ge',0,'breadth_delta','当日样本池上涨占比不低于前日')
    for v in [-1,0]:add(f'market_ge{v}','market_ret','ge',v,'market',f'当日样本池涨跌幅中位数≥{v}%')
    for v in [-2,-4]:add(f'relative_le{v}','relative_ret','le',v,'relative',f'当日弱于样本池涨跌幅中位数至少{-v}%')
    return defs


def clause_mask(d,c):
    v=d[c['field']];z=c['value'];op=c['op']
    if op=='le':r=v<=z+1e-9
    elif op=='lt':r=v<z-1e-9
    elif op=='ge':r=v>=z-1e-9
    elif op=='gt':r=v>z+1e-9
    elif op=='range':r=(v>=z[0]-1e-9)&(v<z[1]-1e-9)
    elif op=='bool':r=v.eq(z)
    else:raise ValueError(op)
    return r.fillna(False).to_numpy(bool)


class Research:
    def __init__(self):
        self.d=pd.read_csv(OUT/'features.csv.gz',dtype={'code':str})
        assert not self.d[['code','date']].duplicated().any()
        self.defs=definitions();self.masks={k:clause_mask(self.d,v) for k,v in self.defs.items()}
        self.dates=sorted(self.d.date.unique());self.day=self.d.date.map({v:i for i,v in enumerate(self.dates)}).to_numpy()
        self.ND=len(self.dates);self.target=self.d.target.to_numpy(float);self.net=self.d.net.to_numpy(float)
        self.anomaly=self.d.next_anomaly.to_numpy(bool)
        self.known=np.array([v in KNOWN_DATES for v in self.dates])
        self.folds={yr:(self.d.date.str.startswith(yr)&self.d.same_year_outcome).to_numpy() for yr in ['2024','2025','2026']}

    def mask(self,parts):
        z=np.ones(len(self.d),bool)
        for c in parts:z&=self.masks[c]
        return z

    def selected(self,mask,cap=5):
        ix=np.flatnonzero(mask)
        if not len(ix):return ix
        ids=self.day[ix];starts=np.flatnonzero(np.r_[True,ids[1:]!=ids[:-1]])
        counts=np.diff(np.r_[starts,len(ix)])
        ranks=np.arange(len(ix))-np.repeat(starts,counts)
        return ix[ranks<cap]

    def metrics(self,mask,cap=5):
        ix=self.selected(mask,cap);n=len(ix)
        if n==0:return {'n':0,'days':0,'score':-1e9}
        raw=np.bincount(self.day[mask],minlength=self.ND)
        count=np.bincount(self.day[ix],minlength=self.ND)
        hit=np.bincount(self.day[ix],weights=self.target[ix],minlength=self.ND)
        pnl=np.bincount(self.day[ix],weights=self.net[ix],minlength=self.ND)
        use=count>0;means=hit[use]/count[use];nets=pnl[use]/count[use]
        date_ids=np.flatnonzero(use)
        top=np.lexsort((np.arange(self.ND),-raw))[:3]
        no3=use.copy();no3[top]=False
        known=use&~self.known
        rate=float(means.mean());stress=float((hit[no3]/count[no3]).mean()) if no3.any() else 0.
        fixed=float((hit[known]/count[known]).mean()) if known.any() else 0.
        se=float(means.std(ddof=1)/np.sqrt(len(means))) if len(means)>1 else 1.
        r={'n':n,'days':int(use.sum()),'target_n':int(hit.sum()),'stock_target_pct':100*hit.sum()/n,
           'day_target_pct':100*rate,'remove3_day_target_pct':100*stress,'known3_removed_day_target_pct':100*fixed,
           'mean_net_pct':float(self.net[ix].mean()),'day_mean_net_pct':float(nets.mean()),
           'loss_ge5_pct':float(100*(self.net[ix]<=-5).mean()),'worst_net_pct':float(self.net[ix].min()),
           'raw_n':int(raw.sum()),'raw_top1_share_pct':100*raw.max()/raw.sum(),
           'raw_top3_share_pct':100*np.sort(raw)[-3:].sum()/raw.sum(),
           'selected_top1_share_pct':100*count.max()/n,'selected_top3_share_pct':100*np.sort(count)[-3:].sum()/n,
           'se_day_pct':100*se,'score':100*(min(rate,stress,fixed)-1.28*se),
           'next_anomaly_n':int(self.anomaly[ix].sum())}
        return r


def dump(path,obj):
    path.write_text(json.dumps(obj,ensure_ascii=False,indent=2))


def train(r):
    defs=r.defs;roots=[k for k,c in defs.items() if c['group']=='root'];atoms=[k for k in defs if k not in roots]
    dump(OUT/'condition-definitions.json',defs)
    settings={'training_year':'2024','root_count':len(roots),'atom_count':len(atoms),'max_extra_conditions':3,
              'beam_per_root':10,'minimum_n':180,'minimum_days':60,'shortlist_max':40,
              'score':'minimum(equal-day hit, remove3 equal-day hit, known3 removed equal-day hit) - 1.28*SE - 0.5pp per extra condition',
              'daily_cap':5,'rank':'entry-day amount descending; code ascending',
              'target':'confirmed net > 2.5; next-price anomalies retained as unconfirmed non-success'}
    dump(OUT/'search-settings.json',settings)
    allrows=[];attempts=0;seen=set();seen_masks=set();beams={}
    def evaluate(parts):
        nonlocal attempts
        parts=tuple([parts[0]]+sorted(parts[1:]))
        if parts in seen:return None
        seen.add(parts);attempts+=1
        mask=r.mask(parts)&r.folds['2024']
        if mask.sum()<180:return None
        fingerprint=hashlib.sha256(np.packbits(mask).tobytes()).hexdigest()
        if fingerprint in seen_masks:return None
        seen_masks.add(fingerprint)
        m=r.metrics(mask)
        if m['n']<180 or m['days']<60:return None
        m['score']-=.5*(len(parts)-1)
        row={'parts':list(parts),'complexity':len(parts),**m};allrows.append(row)
        return row
    for root in roots:
        a=evaluate([root]);beams[root]=[a] if a else []
    for level in range(1,4):
        new={}
        for root,rows in beams.items():
            candidates=[]
            for old in rows:
                used={defs[k]['group'] for k in old['parts']}
                for atom in atoms:
                    if defs[atom]['group'] in used:continue
                    a=evaluate(old['parts']+[atom])
                    if a:candidates.append(a)
            new[root]=sorted(candidates,key=lambda z:(-z['score'],z['complexity'],z['parts']))[:10]
        beams=new
        print('Completed training level',level,'attempts',attempts,'eligible',len(allrows),flush=True)
    ranked=sorted(allrows,key=lambda z:(-z['score'],z['complexity'],z['parts']))
    chosen=[];chosen_ix=[]
    # Diversify predictions on TRAINING only. Baseline roots are added separately.
    for row in ranked:
        ix=set(r.selected(r.mask(row['parts'])&r.folds['2024']))
        if any(len(ix&other)/len(ix|other)>.85 for other in chosen_ix):continue
        chosen.append(row);chosen_ix.append(ix)
        if len(chosen)>=40:break
    for row in allrows:
        if row['complexity']==1 and row not in chosen:chosen.append(row)
    flat=pd.DataFrame(allrows);flat['parts']=flat.parts.map(json.dumps)
    flat.to_csv(OUT/'training-search.csv.gz',index=False)
    payload={'settings':settings,'attempts':attempts,'eligible_unique_training_masks':len(allrows),'shortlist':chosen}
    dump(OUT/'training-shortlist.json',payload)
    print('TRAINING SHORTLIST SAVED',len(chosen),'sha256',hashlib.sha256((OUT/'training-shortlist.json').read_bytes()).hexdigest())
    for row in chosen[:10]:print(row['parts'],{k:round(row[k],3) for k in ['n','days','day_target_pct','remove3_day_target_pct','day_mean_net_pct','score']})


def validate(r):
    source=OUT/'training-shortlist.json';short=json.loads(source.read_text())['shortlist'];rows=[]
    for item in short:
        m=r.metrics(r.mask(item['parts'])&r.folds['2025']);m['score']-=.5*(len(item['parts'])-1)
        enough=m['n']>=100 and m['days']>=40
        positive=m.get('day_mean_net_pct',-999)>0 and item['day_mean_net_pct']>0
        row={'parts':item['parts'],'train':item,'validation':m,'enough_validation':enough,'positive_both':positive,
             'selection_score':min(item['score'],m['score'])}
        rows.append(row)
    dump(OUT/'validation-results.json',rows)
    eligible=[z for z in rows if z['enough_validation'] and z['positive_both']]
    ranked=sorted(eligible,key=lambda z:(-z['selection_score'],len(z['parts']),z['parts']))
    finalists=[];predictions=[]
    for row in ranked:
        ix=set(r.selected(r.mask(row['parts'])&(r.folds['2024']|r.folds['2025'])))
        if any(len(ix&other)/len(ix|other)>.8 for other in predictions):continue
        row['id']=f'F{len(finalists)+1}';finalists.append(row);predictions.append(ix)
        if len(finalists)==3:break
    payload={'shortlist_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),
             'shortlist_n':len(short),'eligible_validation_n':len(eligible),
             'selection':'minimum training/2025 score; both date-equal mean net returns positive; prediction diversity',
             'candidates':finalists,'holdout_checked':False}
    dump(OUT/'frozen-candidates.json',payload)
    print('FROZEN before 2026 audit:',len(finalists),'of',len(short),'shortlist; passing validation:',len(eligible))
    for x in finalists:
        print(x['id'],[r.defs[k]['label'] for k in x['parts']])
        for phase in ['train','validation']:print(phase,x[phase])


if __name__=='__main__':
    phase=sys.argv[1];r=Research()
    if phase=='train':train(r)
    elif phase=='validate':validate(r)
    else:raise ValueError('train or validate')
