"""Prepare explicitly approximate historical pools from fully collected evidence.

Uses unadjusted prior-session close and float shares estimated from that session's
volume/turnover. Turnover rounding and stale suspended prices are disclosed; this
never produces a captured/confirmed pool or modifies daily.gob.
"""
import argparse
import datetime as dt
import hashlib
import json
import math
from pathlib import Path

METHOD = 'prior_close_float_cap_from_volume_turnover'

def read_records(path):
    records = {}
    for line in path.read_text().splitlines():
        item = json.loads(line)
        if 'error' not in item:
            records[item['code']] = item
    return records

def normalized(rows):
    result = []
    previous = ''
    for row in rows:
        if row['date'] <= previous:
            raise ValueError('unordered or duplicate source bar')
        previous = row['date']
        if row['tradestatus'] != '1':
            continue
        values = {k:float(row[k]) for k in ('open','high','low','close','volume','amount')}
        if any(not math.isfinite(v) or v<0 for v in values.values()) or min(values[k] for k in ('open','high','low','close'))<=0:
            raise ValueError('invalid active-session prices')
        if values['high']<max(values['open'],values['close'],values['low']) or values['low']>min(values['open'],values['close']):
            raise ValueError('invalid OHLC range')
        result.append({'Date':row['date'],'Open':values['open'],'High':values['high'],'Low':values['low'],'Close':values['close'],'Volume':values['volume']/100,'VolumeUnit':'hands','AmountYi':values['amount']/1e8,'AmountSource':'baostock_daily','AmountUnit':'yuan','AmountVerified':True})
    return result

def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--evidence',required=True)
    parser.add_argument('--output',required=True)
    args=parser.parse_args()
    root=Path(args.evidence);out=Path(args.output);out.mkdir(parents=True,exist_ok=True)
    universes=json.loads((root/'dated_universes.json').read_text())
    short=read_records(root/'historical_daily.jsonl')
    full=read_records(root/'full_history'/'historical_daily.jsonl')
    records={**short,**full}
    local=json.loads((root/'local_before.json').read_text())
    source_hashes={str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [root/'dated_universes.json',root/'historical_daily.jsonl',root/'full_history'/'historical_daily.jsonl']}
    report={'source_hashes':source_hashes,'dates':{}}
    for date,universe in sorted(universes.items()):
        codes={r['code'] for r in universe}
        if len(codes)!=len(universe):raise ValueError('duplicate universe security')
        missing=sorted(codes-short.keys())
        if missing:raise ValueError(f'{date}: missing raw security responses {missing}')
        reference=max(r['date'] for c in codes for r in records[c]['rows'] if r['date']<date)
        stocks=[];unresolved=[];threshold=[];daily={};calculations=[]
        for stock in universe:
            code=stock['code'];short_code=code[3:]
            before=[r for r in records[code]['rows'] if r['date']<date and float(r.get('turn') or 0)>0 and float(r.get('volume') or 0)>0]
            if not before:
                unresolved.append(code);continue
            bar=before[-1]
            numerator=float(bar['volume'])*float(bar['close'])/1e6
            turn=float(bar['turn'])
            cap=numerator/turn
            low=numerator/(turn+.00005)
            high=numerator/max(turn-.00005,1e-12)
            if low<=50<=high or low<=9000<=high:threshold.append(code)
            if not 50<=cap<=9000:continue
            stocks.append({'Code':code,'ShortCode':short_code,'Name':stock['code_name'],'MarketCapYi':cap})
            calculations.append({'code':code,'price_reference_date':bar['date'],'estimated_float_cap_yi':cap,'rounding_lower_yi':low,'rounding_upper_yi':high})
            if short_code not in local['Dates']:
                if code not in full:raise ValueError(f'{code}: eligible security lacks full historical supplement')
                daily[short_code]=normalized(full[code]['rows'])
        payload={'schema_version':1,'date':date,'reference_date':reference,'generated_at':dt.datetime.now(dt.timezone.utc).isoformat(),'source':'baostock_dated_history','method':METHOD,'evidence_complete':True,'original_snapshot':False,'universe_count':len(universe),'fetched_code_count':len(codes),'stocks':sorted(stocks,key=lambda s:s['ShortCode']),'unresolved_codes':sorted(unresolved),'threshold_uncertain_codes':sorted(threshold),'source_hashes':source_hashes,'daily':daily}
        (out/f'pool_{date}.json').write_text(json.dumps(payload,ensure_ascii=False,indent=2,allow_nan=False)+'\n')
        (root/f'calculations_{date}.json').write_text(json.dumps(calculations,ensure_ascii=False,indent=2,allow_nan=False)+'\n')
        report['dates'][date]={'historical_universe_count':len(universe),'estimated_pool_count':len(stocks),'unresolved_codes':sorted(unresolved),'threshold_uncertain_codes':sorted(threshold),'supplemented_securities':sorted(daily),'supplemented_daily_bars':sum(map(len,daily.values())),'reference_date':reference}
    (root/'reconstruction_report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps(report['dates'],ensure_ascii=False,indent=2))

if __name__=='__main__':main()
