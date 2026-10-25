"""Reproducible local bundles; never publish a tag or release."""
import hashlib, json, os, pathlib, shutil, subprocess, tarfile, zipfile

ROOT = pathlib.Path(__file__).resolve().parents[1]
DIST = ROOT / 'dist'
GO = os.environ.get('GO_BINARY', 'go')
VERSION = (ROOT/'CONSOLE_VERSION').read_text().strip()
def run(*args, env=None):
    return subprocess.check_output([GO,*args], cwd=ROOT, env=env, text=True, encoding='utf-8')
def build():
    DIST.mkdir(exist_ok=True)
    targets=[('windows','amd64'),('linux','amd64'),('linux','arm64')]
    modules=run('list','-m','-json','all')
    (DIST/'modules.json').write_text(modules,encoding='utf-8')
    decoder=json.JSONDecoder(); remaining=modules; components=[]
    while remaining.strip():
        item,end=decoder.raw_decode(remaining.lstrip());remaining=remaining.lstrip()[end:]
        components.append({'type':'library','name':item['Path'],'version':item.get('Version','workspace')})
    components.append({'type':'library','name':'flag-icons','version':'7.6.0','licenses':[{'license':{'id':'MIT'}}]})
    sbom={'bomFormat':'CycloneDX','specVersion':'1.5','version':1,'components':components}
    (DIST/'sbom.cdx.json').write_text(json.dumps(sbom,indent=2),encoding='utf-8')
    source_paths=subprocess.check_output(['git','ls-files','-z','--cached','--others','--exclude-standard'],cwd=ROOT).decode('utf-8').split('\0')
    source_manifest=''.join(f'{hashlib.sha256((ROOT/p).read_bytes()).hexdigest()}  {p}\n' for p in sorted(set(source_paths)) if p and (ROOT/p).is_file())
    (DIST/'SOURCE-SHA256SUMS').write_text(source_manifest,encoding='utf-8')
    helper_assets=DIST/'helper-assets';helper_assets.mkdir(exist_ok=True)
    manifest={}
    for arch in ['amd64','arm64']:
        name=f'greflow-agent-linux-{arch}'
        run('build','-p','2','-trimpath','-ldflags=-s -w','-o',str(helper_assets/name),'./cmd/greflow-agent',env={**os.environ,'GOOS':'linux','GOARCH':arch,'CGO_ENABLED':'0'})
        manifest[name]=hashlib.sha256((helper_assets/name).read_bytes()).hexdigest()
    (helper_assets/'manifest.json').write_text(json.dumps(manifest,indent=2),encoding='utf-8')
    for platform,arch in targets:
        env={**os.environ,'GOOS':platform,'GOARCH':arch,'CGO_ENABLED':'0'}
        bundle=DIST/f'greflow-console-{platform}-{arch}';bundle.mkdir(exist_ok=True)
        shutil.copytree(helper_assets,bundle/'helpers',dirs_exist_ok=True)
        for program in (['greflow-console'] if platform=='windows' else ['greflow-console','greflow-agent','greflow']):
            filename=program+('.exe' if platform=='windows' else '')
            print(f'Building {platform}/{arch} {program}',flush=True)
            ldflags='-s -w' + (' -X main.version='+VERSION if program=='greflow-console' else '')
            run('build','-p','2','-trimpath','-ldflags='+ldflags,'-o',str(bundle/filename),'./cmd/'+program,env=env)
        files=['Start-GreFlow.ps1','Stop-GreFlow.ps1'] if platform=='windows' else ['install-console.sh','install-agent.sh','uninstall-console.sh','greflow-console.service','greflow-agent.service','nginx.conf.example']
        for f in files:
            if platform=='linux':
                (bundle/f).write_text((ROOT/'packaging'/f).read_text(encoding='utf-8'),encoding='utf-8',newline='\n')
            else:shutil.copy2(ROOT/'packaging'/f,bundle/f)
        shutil.copy2(ROOT/'LICENSE',bundle/'LICENSE')
        shutil.copy2(ROOT/'web/app/assets/OFL.txt',bundle/'FONT-LICENSE.txt')
        shutil.copy2(ROOT/'web/app/assets/FLAGS-LICENSE.txt',bundle/'FLAGS-LICENSE.txt')
        shutil.copy2(ROOT/'docs/console/RUN.fa.md',bundle/'RUN.fa.md')
        shutil.copy2(ROOT/'docs/console/USAGE.fa.md',bundle/'USAGE.fa.md')
        shutil.copy2(ROOT/'docs/console/TROUBLESHOOTING.fa.md',bundle/'TROUBLESHOOTING.fa.md')
        shutil.copy2(ROOT/'docs/console/TEST-RESULTS.md',bundle/'TEST-RESULTS.md')
        shutil.copy2(DIST/'sbom.cdx.json',bundle/'sbom.cdx.json')
        shutil.copy2(DIST/'SOURCE-SHA256SUMS',bundle/'SOURCE-SHA256SUMS')
        license_dir=bundle/'licenses';license_dir.mkdir(exist_ok=True)
        remaining=modules
        while remaining.strip():
            item,end=decoder.raw_decode(remaining.lstrip());remaining=remaining.lstrip()[end:]
            if 'Dir' not in item:continue
            for filename in ['LICENSE','LICENSE.txt','COPYING']:
                src=pathlib.Path(item['Dir'])/filename
                if src.is_file():
                    dest=license_dir/(item['Path'].replace('/','_')+'-'+filename)
                    if dest.exists():dest.chmod(0o644)
                    shutil.copyfile(src,dest)
                    break
        dirty=bool(subprocess.check_output(['git','status','--porcelain'],cwd=ROOT,text=True).strip())
        (bundle/'BUILD.txt').write_text(run('version')+'Source: '+subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip()+(' + working tree' if dirty else '')+'\nConsole: '+VERSION+'\nSee TEST-RESULTS.md and release notes for validation scope.\n',encoding='utf-8')
        checks=''.join(f'{hashlib.sha256(p.read_bytes()).hexdigest()}  {p.relative_to(bundle).as_posix()}\n' for p in sorted(bundle.rglob('*')) if p.is_file() and p.name!='SHA256SUMS')
        (bundle/'SHA256SUMS').write_text(checks,encoding='utf-8')
        if platform=='windows':
            with zipfile.ZipFile(str(bundle)+'.zip','w',zipfile.ZIP_DEFLATED) as z:
                for p in bundle.rglob('*'):
                    if p.is_file():z.write(p,p.relative_to(bundle))
        else:
            with tarfile.open(str(bundle)+'.tar.gz','w:gz') as z:
                def permissions(info):
                    info.uid=info.gid=0;info.uname=info.gname='root';info.mtime=0
                    info.mode=0o755 if info.isdir() or info.name.endswith('.sh') or pathlib.PurePosixPath(info.name).name in ['greflow','greflow-agent','greflow-console'] else 0o644
                    return info
                z.add(bundle,arcname=bundle.name,filter=permissions)
    archives=sorted([*DIST.glob('greflow-console-*.zip'),*DIST.glob('greflow-console-*.tar.gz')])
    (DIST/'CONSOLE-SHA256SUMS').write_text(''.join(f'{hashlib.sha256(p.read_bytes()).hexdigest()}  {p.name}\n' for p in archives),encoding='utf-8')
if __name__=='__main__':build()
