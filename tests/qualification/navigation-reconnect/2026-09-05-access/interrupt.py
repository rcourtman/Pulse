import subprocess, time, re, pathlib, signal, json, os
root=pathlib.Path.cwd(); out=root/'tmp/web-followup'; out.mkdir(parents=True, exist_ok=True); log=out/'interruption.log'
with log.open('w') as f:
 p=subprocess.Popen(['bash','tests/integration/scripts/run-tests.sh','multi-tenant'],stdout=f,stderr=subprocess.STDOUT)
 run=None; before=None; interrupted=False
 try:
  deadline=time.time()+1200
  while p.poll() is None and time.time()<deadline:
   data=log.read_text(); m=re.search(r'Isolated integration project: (pulse-e2e-[a-f0-9]+)',data)
   if m: run=m[1]
   if run:
    auth=root/'tests/integration/tmp/playwright-auth'/run/'shared-cookie-session.json'
    procs=subprocess.check_output(['ps','-eo','pid,ppid,args'],text=True)
    # Require browser descendant and completed cookie bootstrap before signalling.
    descendants={p.pid}; rows=[]
    for line in procs.splitlines()[1:]:
     fields=line.strip().split(None,2)
     if len(fields)==3: rows.append((int(fields[0]),int(fields[1]),fields[2]))
    for _ in range(12):
     descendants.update(pid for pid,ppid,args in rows if ppid in descendants)
    browsers=[pid for pid,ppid,args in rows if pid in descendants and ('headless_shell' in args or '/chrome ' in args)]
    if auth.exists() and browsers:
     before=subprocess.check_output(['docker','ps','-a','--filter',f'label=com.docker.compose.project={run}','--format','{{.Names}} {{.State}}'],text=True)
     if 'running' in before:
      p.send_signal(signal.SIGTERM); interrupted=True; break
   time.sleep(.2)
  if not interrupted and p.poll() is None: p.send_signal(signal.SIGTERM)
  code=p.wait(timeout=60)
  after=subprocess.check_output(['docker','ps','-a','--filter',f'label=com.docker.compose.project={run}','--format','{{.Names}}'],text=True) if run else 'no run'
  volumes=subprocess.check_output(['docker','volume','ls','--filter',f'label=com.docker.compose.project={run}','--format','{{.Name}}'],text=True) if run else 'no run'
  remaining=[pid for pid in descendants if pathlib.Path(f'/proc/{pid}').exists()] if interrupted else []
  paths=[root/'tests/integration'/part/run for part in ['tmp/playwright-auth','test-results','playwright-report']] if run else []
  result=dict(run=run,interrupted=interrupted,exit=code,containers_before=before,containers_after=after,volumes_after=volumes,remaining_pids=remaining,owned_paths_remaining=[str(x.relative_to(root)) for x in paths if x.exists()])
  (out/'interruption.json').write_text(json.dumps(result,indent=2)+'\n'); print(json.dumps(result,indent=2))
  assert interrupted and code == 143, 'Did not interrupt authenticated Chromium'
  assert not after and not volumes and not remaining and not result['owned_paths_remaining'], 'Owned resources remain'
 finally:
  if p.poll() is None: p.send_signal(signal.SIGTERM); p.wait(timeout=60)
