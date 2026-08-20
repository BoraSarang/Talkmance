#!/usr/bin/env python3
"""톡맨스 아이콘 → macOS icns 세트 생성"""

import os, subprocess, shutil
from PIL import Image

SRC = "docs/screenshots/talkmance_icon.png"
ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
OUT_MAC = os.path.join(ROOT, "assets", "icon", "macos")

def save_resized(src, path, size):
    im = Image.open(src).convert("RGBA").resize((size, size), Image.LANCZOS)
    im.save(path, "PNG")
    return im

# ---------- 1. macOS iconset ----------
iconset = os.path.join(OUT_MAC, "Talkmance.iconset")
os.makedirs(iconset, exist_ok=True)
mac_sizes = {
    "icon_16x16.png": 16, "icon_16x16@2x.png": 32,
    "icon_32x32.png": 32, "icon_32x32@2x.png": 64,
    "icon_128x128.png": 128, "icon_128x128@2x.png": 256,
    "icon_256x256.png": 256, "icon_256x256@2x.png": 512,
    "icon_512x512.png": 512, "icon_512x512@2x.png": 1024,
}
for name, size in mac_sizes.items():
    save_resized(SRC, os.path.join(iconset, name), size)

icns_path = os.path.join(OUT_MAC, "Talkmance.icns")
subprocess.run(["iconutil", "-c", "icns", iconset, "-o", icns_path], check=True)
shutil.rmtree(iconset)
print("macOS icns:", icns_path, f"({os.path.getsize(icns_path)//1024}KB)")

# ---------- 2. macOS Xcode Assets (AppIcon + MenuBar 템플릿) ----------
OUT_XC = os.path.join(ROOT, "apps", "macos", "Talkmance", "Assets.xcassets")

def write_json(path, data):
    with open(path, "w") as f:
        import json
        json.dump(data, f, indent=2, ensure_ascii=False)

# 4-1. AppIcon.appiconset (icns와 동일 크기 세트)
appicon_dir = os.path.join(OUT_XC, "AppIcon.appiconset")
os.makedirs(appicon_dir, exist_ok=True)
for name, size in mac_sizes.items():
    save_resized(SRC, os.path.join(appicon_dir, name), size)
appicon_contents = {
    "images": [
        {"filename": "icon_16x16.png", "idiom": "mac", "scale": "1x", "size": "16x16"},
        {"filename": "icon_16x16@2x.png", "idiom": "mac", "scale": "2x", "size": "16x16"},
        {"filename": "icon_32x32.png", "idiom": "mac", "scale": "1x", "size": "32x32"},
        {"filename": "icon_32x32@2x.png", "idiom": "mac", "scale": "2x", "size": "32x32"},
        {"filename": "icon_128x128.png", "idiom": "mac", "scale": "1x", "size": "128x128"},
        {"filename": "icon_128x128@2x.png", "idiom": "mac", "scale": "2x", "size": "128x128"},
        {"filename": "icon_256x256.png", "idiom": "mac", "scale": "1x", "size": "256x256"},
        {"filename": "icon_256x256@2x.png", "idiom": "mac", "scale": "2x", "size": "256x256"},
        {"filename": "icon_512x512.png", "idiom": "mac", "scale": "1x", "size": "512x512"},
        {"filename": "icon_512x512@2x.png", "idiom": "mac", "scale": "2x", "size": "512x512"},
    ],
    "info": {"author": "xcode", "version": 1},
}
write_json(os.path.join(appicon_dir, "Contents.json"), appicon_contents)

# 4-2. MenuBarIcon.imageset — 말풍선+하트 추출 후 검정 템플릿 (16/32px)
def extract_subject(src_path):
    """흰색 말풍선 + 핑크 하트만 남기고 나머지 투명 처리"""
    im = Image.open(src_path).convert("RGBA")
    px = im.load()
    for y in range(im.height):
        for x in range(im.width):
            r, g, b, a = px[x, y]
            is_white = r > 235 and g > 235 and b > 235
            is_heart = r > 200 and g < 160 and 80 < b < 180
            if not (is_white or is_heart):
                px[x, y] = (0, 0, 0, 0)
    return im

def make_template_icon(src_path, size):
    """흰 말풍선+하트 추출 → 검정 템플릿 (alpha 전용)"""
    subject = extract_subject(src_path)
    # 그레이스케일 → 검정: 색 제거하고 알파 유지
    subject = subject.convert("RGBA")
    px = subject.load()
    for y in range(subject.height):
        for x in range(subject.width):
            r, g, b, a = px[x, y]
            if a > 0:
                px[x, y] = (0, 0, 0, a)
    # 크기 조정 시 안티앨리어싱 알파 유지
    im = subject.resize((size, size), Image.LANCZOS)
    return im

menubar_dir = os.path.join(OUT_XC, "MenuBarIcon.imageset")
os.makedirs(menubar_dir, exist_ok=True)
subject_full = extract_subject(SRC)
# 말풍선+하트가 화면 86% 폭이므로 여백 없이 bbox로 크롭해 아이콘을 키움
bbox = subject_full.getchannel("A").getbbox()
cropped = subject_full.crop(bbox)
base = max(cropped.size)
canvas = Image.new("RGBA", (base, base), (0, 0, 0, 0))
canvas.paste(cropped, ((base - cropped.width) // 2, (base - cropped.height) // 2), cropped)
menu_16 = canvas.resize((16, 16), Image.LANCZOS)
menu_32 = canvas.resize((32, 32), Image.LANCZOS)
menu_16.save(os.path.join(menubar_dir, "menubar_16.png"))
menu_32.save(os.path.join(menubar_dir, "menubar_16@2x.png"))
write_json(os.path.join(menubar_dir, "Contents.json"), {
    "images": [
        {"filename": "menubar_16.png", "idiom": "mac", "scale": "1x"},
        {"filename": "menubar_16@2x.png", "idiom": "mac", "scale": "2x"},
    ],
    "info": {"author": "xcode", "version": 1},
    "properties": {"template-rendering-intent": "template"},
})

# 4-3. AccentColor.colorset — 로맨스 핑크
accent_dir = os.path.join(OUT_XC, "AccentColor.colorset")
os.makedirs(accent_dir, exist_ok=True)
write_json(os.path.join(accent_dir, "Contents.json"), {
    "colors": [
        {
            "color": {"color-space": "srgb", "components": {"alpha": "1.000", "blue": "0.467", "green": "0.200", "red": "1.000"}},
            "idiom": "universal",
        }
    ],
    "info": {"author": "xcode", "version": 1},
})

# 루트 Contents.json
write_json(os.path.join(OUT_XC, "Contents.json"), {
    "info": {"author": "xcode", "version": 1},
})

print("\nmacOS Xcode Assets:")
for root, dirs, files in os.walk(OUT_XC):
    for f in sorted(files):
        p = os.path.join(root, f)
        print(" ", os.path.relpath(p, ROOT))