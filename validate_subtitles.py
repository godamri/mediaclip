#!/usr/bin/env python3
"""Subtitle timeline validator.
Asserts no more than one subtitle cue is active at any timestamp
for LINE/WORD/KARAOKE modes.
"""

import re, sys, os

def parse_time(t):
    h, m, s = t.split(':')
    return int(h) * 3600 + int(m) * 60 + float(s)

def parse_ass(path):
    events = []
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line.startswith('Dialogue:'):
                continue
            parts = line.split(',', 9)
            if len(parts) < 10:
                continue
            start = parse_time(parts[1].strip())
            end = parse_time(parts[2].strip())
            layer = int(parts[0].replace('Dialogue:', '').strip())
            style = parts[3].strip()
            text = parts[9]
            events.append({
                'layer': layer,
                'start': round(start, 3),
                'end': round(end, 3),
                'style': style,
                'text': text[:80],
                'source': path,
            })
    return events

def validate(events, label):
    print(f"\n{'='*60}")
    print(f"VALIDATE: {label}")
    print(f"{'='*60}")
    
    events.sort(key=lambda e: (e['start'], e['end']))
    
    is_multi_layer = len(set(e['layer'] for e in events)) > 1
    all_ok = True
    
    # 1. Check start < end
    for e in events:
        if e['start'] >= e['end']:
            print(f"  FAIL: zero/negative duration: {e['start']:.3f}-{e['end']:.3f} {e['text']}")
            all_ok = False
    
    # 2. Check for overlaps within each layer
    layers = set(e['layer'] for e in events)
    for layer in sorted(layers):
        layer_events = [e for e in events if e['layer'] == layer]
        layer_events.sort(key=lambda e: (e['start'], e['end']))
        
        overlap_count = 0
        for i in range(len(layer_events) - 1):
            a, b = layer_events[i], layer_events[i+1]
            if a['end'] > b['start'] + 0.002:
                overlap_count += 1
                if overlap_count <= 3:
                    print(f"  FAIL Layer {layer}: overlap")
                    print(f"    A: {a['start']:.3f}-{a['end']:.3f} {a['text'][:50]}")
                    print(f"    B: {b['start']:.3f}-{b['end']:.3f} {b['text'][:50]}")
                    all_ok = False
        
        if overlap_count == 0:
            print(f"  Layer {layer}: {len(layer_events)} events, no overlaps ✓")
        else:
            print(f"  Layer {layer}: {len(layer_events)} events, {overlap_count} overlaps ✗")
    
    # 3. Max active per layer (KARAOKE multi-layer: one per layer is OK)
    for layer in sorted(layers):
        layer_events = [e for e in events if e['layer'] == layer]
        timestamps = set()
        for e in layer_events:
            timestamps.add(e['start'])
            timestamps.add(e['end'])
        
        max_per_layer = 0
        worst_t = 0
        for t in sorted(timestamps):
            active = sum(1 for e in layer_events if e['start'] <= t < e['end'])
            if active > max_per_layer:
                max_per_layer = active
                worst_t = t
        
        status = "✓" if max_per_layer <= 1 else "✗"
        if max_per_layer > 1:
            all_ok = False
            print(f"  FAIL Layer {layer}: {max_per_layer} active at t={worst_t:.3f}s {status}")
            for e in layer_events:
                if e['start'] <= worst_t < e['end']:
                    print(f"    ACTIVE: {e['start']:.3f}-{e['end']:.3f} {e['text'][:50]}")
        else:
            print(f"  Layer {layer}: max 1 active (perfect) ✓")
    
    # 4. Cross-layer check — only fail if >1 in SAME layer
    total_layers = len(layers)
    if total_layers > 1:
        print(f"  Multi-layer ({total_layers} layers): {len(events)} total events")
        print(f"  Cross-layer active count not checked (expected for karaoke)")
    
    return all_ok

if __name__ == '__main__':
    clip_dir = os.path.join(os.path.dirname(__file__) or '.', 'demo')
    
    results = []
    for fname in ['sub_line.ass', 'sub_word.ass', 'sub_karaoke.ass', 'sub_karaoke_acc.ass', 'sub_karaoke_dynamic.ass']:
        path = os.path.join(clip_dir, fname)
        if not os.path.exists(path):
            print(f"\nSKIP: {fname} not found")
            continue
        events = parse_ass(path)
        ok = validate(events, fname)
        results.append((fname, len(events), ok))
    
    print(f"\n{'='*60}")
    print("SUMMARY")
    print(f"{'='*60}")
    all_pass = True
    for name, count, ok in results:
        status = "PASS" if ok else "FAIL"
        if not ok:
            all_pass = False
        print(f"  {status:4s} {count:3d} events  {name}")
    
    sys.exit(0 if all_pass else 1)
