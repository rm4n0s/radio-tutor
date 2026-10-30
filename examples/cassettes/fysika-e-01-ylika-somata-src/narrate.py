#!/usr/bin/env python3
"""Makes the narration (narration.ogg) and the timeline (lesson.json) of the lessons of this cassette.

Usage: narrate.py <src folder> <cassette folder> [--piper PATH] [--model PATH] [--tmp DIR]

The sources (this folder, next to the cassette folder):
  script.json          the sentences to speak and the pauses between them
  lessons/<id>.json    a template of lesson.json: title, summary and tracks. A key gives its time
                       as "t" (seconds) or as "at", an anchor to the speech: "s3" is the start of
                       sentence 3 (counting from 1), "e3" its end, "end" the end of the narration,
                       each with an optional offset ("s3+0.5", "e2-0.2").

Every sentence is spoken with piper (voice el_GR-rapunzelina-low, a little slower than normal: length_scale in script.json), converted to mono 22050 Hz,
measured with ffprobe, joined with ffmpeg with the pauses of script.json and encoded as Ogg Vorbis.
The captions are the sentences, with the measured times; the anchors of the template are
resolved against those times. Needs piper, ffmpeg and ffprobe.
"""
import argparse, hashlib, json, math, os, re, subprocess, sys, tempfile

def run(*cmd, stdin=None):
    r = subprocess.run(cmd, input=stdin, capture_output=True)
    if r.returncode != 0:
        sys.exit('%s failed: %s' % (cmd[0], r.stderr.decode(errors='replace')[-500:]))
    return r.stdout.decode()

def duration(path):
    return float(run('ffprobe', '-v', 'error', '-show_entries', 'format=duration', '-of', 'csv=p=0', path).strip())

def rnd(v):
    return round(v * 1000) / 1000

ANCHOR = re.compile(r'^(?:s(\d+)|e(\d+)|(end))([+-]\d+(?:\.\d+)?)?$')

def resolve(anchor, spans, total):
    m = ANCHOR.match(anchor)
    if not m:
        sys.exit('bad anchor ' + anchor)
    if m.group(3):
        base = total
    else:
        n = int(m.group(1) or m.group(2))
        if not 1 <= n <= len(spans):
            sys.exit('anchor names a missing sentence: ' + anchor)
        base = spans[n - 1][0] if m.group(1) else spans[n - 1][1]
    return rnd(base + float(m.group(4) or 0))

def narrate(sc, item, work, out_ogg, piper, model):
    d = os.path.join(work, item['id'])
    os.makedirs(d, exist_ok=True)
    durs, wavs = [], []
    for i, s in enumerate(item['sentences']):
        ls = str(sc.get('length_scale', 1.0))
        h = hashlib.sha1((model + ls + s['text']).encode()).hexdigest()[:12]
        raw, wav = os.path.join(d, 'raw-%s.wav' % h), os.path.join(d, 's-%s.wav' % h)
        if not os.path.exists(wav):
            run(piper, '--model', model, '--length-scale', ls, '--output_file', raw, stdin=(s['text'] + '\n').encode())
            run('ffmpeg', '-v', 'error', '-y', '-i', raw, '-ac', '1', '-ar', '22050', '-c:a', 'pcm_s16le', wav)
        wavs.append(wav)
        durs.append(duration(wav))
    spans, t, lst = [], sc['lead'], []
    def silence(sec):
        name = os.path.join(d, 'silence-%d.wav' % round(sec * 1000))
        if not os.path.exists(name):
            run('ffmpeg', '-v', 'error', '-y', '-f', 'lavfi', '-i', 'anullsrc=r=22050:cl=mono', '-t', '%.3f' % sec, '-c:a', 'pcm_s16le', name)
        lst.append("file '%s'" % name)
    silence(sc['lead'])
    for i, (w, dur) in enumerate(zip(wavs, durs)):
        spans.append((rnd(t), rnd(t + dur)))
        lst.append("file '%s'" % w)
        t += dur
        gap = sc['tail'] if i == len(wavs) - 1 else item['sentences'][i].get('pause', sc['silence'])
        silence(gap)
        t += gap
    total = rnd(t)
    lf = os.path.join(d, 'list.txt')
    open(lf, 'w').write('\n'.join(lst) + '\n')
    os.makedirs(os.path.dirname(out_ogg), exist_ok=True)
    run('ffmpeg', '-v', 'error', '-y', '-f', 'concat', '-safe', '0', '-i', lf, '-ac', '1', '-ar', '22050',
        '-map_metadata', '-1', '-fflags', '+bitexact', '-flags:a', '+bitexact', '-c:a', 'libvorbis', '-q:a', '2', out_ogg)
    if duration(out_ogg) < total - 0.05:
        sys.exit('encoded narration is shorter than planned: ' + item['id'])
    return spans, total

def lesson_json(tm, item, spans, total):
    J = lambda v: json.dumps(v, ensure_ascii=False, separators=(',', ':'))
    out = ['{', '  "format": 1,', '  "title": %s,' % J(tm['title'])]
    if tm.get('summary'):
        out.append('  "summary": %s,' % J(tm['summary']))
    out.append('  "tracks": [')
    tr = []
    for t in tm['tracks']:
        s = ['    {', '      "actor": %s,' % J(t['actor'])]
        for f in ('origin', 'reveal_box', 'reveal_from'):
            if f in t:
                s.append('      "%s": %s,' % (f, J(t[f])))
        ks = []
        for k in t['keys']:
            kk = {'t': resolve(k['at'], spans, total) if 'at' in k else k['t']}
            if kk['t'] > total:
                sys.exit('key after the end: %s %s' % (t['actor'], kk['t']))
            for p in ('opacity', 'x', 'y', 'scale', 'rotate', 'reveal', 'ease'):
                if p in k:
                    v = k[p]
                    kk[p] = round(v, 3) if isinstance(v, float) else v
            ks.append('        ' + J(kk))
        s.append('      "keys": [\n' + ',\n'.join(ks) + '\n      ]\n    }')
        tr.append('\n'.join(s))
    out.append(',\n'.join(tr))
    out.append('  ],')
    caps = ['    ' + J({'start': spans[i][0], 'end': spans[i][1], 'text': s['text']}) for i, s in enumerate(item['sentences'])]
    out.append('  "captions": [\n' + ',\n'.join(caps) + '\n  ]')
    out.append('}')
    return '\n'.join(out) + '\n'

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('src'); ap.add_argument('out')
    ap.add_argument('--piper', default=os.path.expanduser('~/piper/bin/piper'))
    ap.add_argument('--model', default=os.path.expanduser('~/piper/el_GR-rapunzelina-low.onnx'))
    ap.add_argument('--tmp'); ap.add_argument('--spans'); ap.add_argument('--only')
    a = ap.parse_args()
    sc = json.load(open(os.path.join(a.src, 'script.json'), encoding='utf-8'))
    work = a.tmp or tempfile.mkdtemp(prefix='narrate-')
    for item in sc['lessons']:
        if a.only and item['id'] != a.only:
            continue
        spans, total = narrate(sc, item, work, os.path.join(a.out, 'lessons', item['id'], 'narration.ogg'), a.piper, a.model)
        tp = os.path.join(a.src, 'lessons', item['id'] + '.json')
        if os.path.exists(tp):
            tm = json.load(open(tp, encoding='utf-8'))
            open(os.path.join(a.out, 'lessons', item['id'], 'lesson.json'), 'w', encoding='utf-8').write(lesson_json(tm, item, spans, total))
        if a.spans:
            os.makedirs(a.spans, exist_ok=True)
            json.dump({'spans': spans, 'total': total}, open(os.path.join(a.spans, item['id'] + '.json'), 'w'))
        print('lesson %s: %.1f s' % (item['id'], total))
        for i, s in enumerate(item['sentences']):
            print('  s%d %6.2f - %6.2f  %s' % (i + 1, spans[i][0], spans[i][1], s['text']))

main()
