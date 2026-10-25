"""Collect dated BaoStock evidence, resumably; never writes official pool caches."""
import argparse
import concurrent.futures
import contextlib
import datetime as dt
import io
import json
import multiprocessing as mp
from pathlib import Path
import signal
import socket
import time

FIELDS = 'date,code,open,high,low,close,volume,amount,turn,tradestatus,isST'

def timeout(*_):
    raise TimeoutError('provider query exceeded 25 seconds')

def login():
    import baostock as bs
    import baostock.common.context as context
    import baostock.util.socketutil as socketutil
    # The SDK silently ends pagination when transport returns None. Escalate it
    # so a partial historical universe can never pass as a successful download.
    if not getattr(socketutil, '_strict_evidence_transport', False):
        original_send = socketutil.send_msg
        def checked_send(message):
            response = original_send(message)
            if response is None:
                raise ConnectionError('provider transport returned no complete response')
            return response
        socketutil.send_msg = checked_send
        socketutil._strict_evidence_transport = True
    old_socket = getattr(context, 'default_socket', None)
    if old_socket is not None:
        old_socket.close()
    socket.setdefaulttimeout(12)
    signal.signal(signal.SIGALRM, timeout)
    signal.alarm(25)
    try:
        with contextlib.redirect_stdout(io.StringIO()):
            result = bs.login()
        if result.error_code != '0':
            raise RuntimeError(f'{result.error_code}: {result.error_msg}')
    finally:
        signal.alarm(0)

def fetch(task):
    code, start_date, end_date = task
    import baostock as bs
    for attempt in range(2):
        started = time.monotonic()
        try:
            signal.alarm(25)
            result = bs.query_history_k_data_plus(code, FIELDS, start_date=start_date, end_date=end_date, frequency='d', adjustflag='3')
            rows = []
            while result.error_code == '0' and result.next():
                rows.append(dict(zip(result.fields, result.get_row_data())))
            if result.error_code != '0':
                raise RuntimeError(f'{result.error_code}: {result.error_msg}')
            return {'code': code, 'rows': rows, 'query_start':start_date,'query_end':end_date,'elapsed': round(time.monotonic()-started, 3), 'fetched_at': dt.datetime.now(dt.timezone.utc).isoformat()}
        except Exception as exc:
            error = str(exc)
            if attempt == 0:
                try:
                    login()
                except Exception as reconnect:
                    error += '; reconnect: '+str(reconnect)
        finally:
            signal.alarm(0)
    return {'code': code, 'error': error}

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--output', required=True)
    parser.add_argument('--workers', type=int, default=4)
    parser.add_argument('--codes-file', help='optional JSON list of dated or locally stored security codes')
    parser.add_argument('--start', default='2026-09-25')
    parser.add_argument('--end', default='2026-10-23')
    args = parser.parse_args()
    output = Path(args.output)
    output.mkdir(parents=True, exist_ok=True)
    universe_path = output/'dated_universes.json'
    if args.codes_file:
        universes = {'explicit_code_list': [{'code': c} for c in json.loads(Path(args.codes_file).read_text())]}
    elif universe_path.exists():
        universes = json.loads(universe_path.read_text())
    else:
        import baostock as bs
        login()
        universes = {}
        for day in ['2026-10-08', '2026-10-09', '2026-10-20']:
            for attempt in range(3):
                signal.alarm(45)
                try:
                    result = bs.query_all_stock(day=day)
                    rows = []
                    while result.error_code == '0' and result.next():
                        row = dict(zip(result.fields, result.get_row_data()))
                        if row['code'].startswith(('sh.60', 'sz.00')):
                            rows.append(row)
                    if result.error_code != '0' or len(rows)<2000:
                        raise RuntimeError(f'incomplete historical universe {day}: {result.error_code} {len(rows)}')
                    universes[day] = rows
                    print(json.dumps({'date': day, 'mainboard_codes': len(rows)}), flush=True)
                    break
                except Exception:
                    if attempt == 2:
                        raise
                    login()
                finally:
                    signal.alarm(0)
        universe_path.write_text(json.dumps(universes, ensure_ascii=False, indent=2)+'\n')
        with contextlib.redirect_stdout(io.StringIO()):
            bs.logout()
    codes = sorted({r['code'] for rows in universes.values() for r in rows})
    raw_path = output/'historical_daily.jsonl'
    done = set()
    if raw_path.exists():
        for line in raw_path.read_text().splitlines():
            row = json.loads(line)
            if 'error' not in row and row.get('query_start', '2026-09-25') <= args.start and row.get('query_end', '2026-10-23') >= args.end:
                done.add(row['code'])
    pending = [code for code in codes if code not in done]
    print(json.dumps({'total':len(codes),'already_done':len(done),'pending':len(pending)}),flush=True)
    with raw_path.open('a') as handle, concurrent.futures.ProcessPoolExecutor(max_workers=max(1,min(4,args.workers)),mp_context=mp.get_context('spawn'),initializer=login) as executor:
        futures = [executor.submit(fetch, (code,args.start,args.end)) for code in pending]
        errors = 0
        for count,future in enumerate(concurrent.futures.as_completed(futures),1):
            row = future.result()
            handle.write(json.dumps(row,ensure_ascii=False)+'\n')
            handle.flush()
            errors += 'error' in row
            if count%100==0 or count==len(pending):
                print(json.dumps({'processed':count,'pending_total':len(pending),'errors':errors}),flush=True)
    print('Evidence collected; no official archive was changed.',flush=True)

if __name__ == '__main__':
    main()
