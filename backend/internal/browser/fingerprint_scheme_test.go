package browser

import (
	"strings"
	"testing"
)

func TestApplySchemeToArgsKeepsDeviceAndForcesFixed(t *testing.T) {
	t.Parallel()
	scheme := BuiltinFingerprintSchemes()[0]
	existing := []string{
		"--fingerprint=2031806230", "--fingerprint-brand=Chrome", "--fingerprint-platform=windows",
		"--lang=en-US", "--timezone=America/Denver", "--window-size=1600,900",
		"--fingerprint-color-depth=24", "--fingerprint-hardware-concurrency=6", "--fingerprint-device-memory=8",
		"--fingerprint-canvas-noise=true", "--webrtc-ip-handling-policy=default_public_interface_only",
	}
	got := ApplySchemeToArgs(&scheme, "profile-a", existing)
	joined := " " + strings.Join(got, " ") + " "
	for _, want := range []string{"--fingerprint=2031806230", "--fingerprint-hardware-concurrency=6", "--timezone=America/Los_Angeles", "--lang=en-US", "--fingerprint-brand=Chrome", "--webrtc-ip-handling-policy=disable_non_proxied_udp", "--disable-features=WebGPUService", "--disable-speech-api", "--start-maximized"} {
		if !strings.Contains(joined, " "+want+" ") {
			t.Fatalf("缺少 %s，实际：%v", want, got)
		}
	}
	for _, bad := range []string{"America/Denver", "--fingerprint-color-depth", "--fingerprint-canvas-noise", "--window-size=", "default_public_interface_only"} {
		if strings.Contains(joined, bad) {
			t.Fatalf("不应保留 %s，实际：%v", bad, got)
		}
	}
	if strings.Count(joined, "--timezone=") != 1 {
		t.Fatalf("时区参数应只出现一次：%v", got)
	}
}

func TestApplySchemeToArgsFillsMissingDeviceDeterministically(t *testing.T) {
	t.Parallel()
	scheme := BuiltinFingerprintSchemes()[0]
	first := ApplySchemeToArgs(&scheme, "profile-b", []string{"--lang=en-US", "--timezone=America/Los_Angeles"})
	second := ApplySchemeToArgs(&scheme, "profile-b", []string{"--lang=en-US", "--timezone=America/Los_Angeles"})
	if strings.Join(first, "|") != strings.Join(second, "|") {
		t.Fatalf("同一实例两次结果应一致：\n%v\n%v", first, second)
	}
	if ExtractFingerprintSeed(first) != LegacyFallbackSeed("profile-b") {
		t.Fatalf("缺种子时应沿用旧版回退种子，实际 %q", ExtractFingerprintSeed(first))
	}
	other := ApplySchemeToArgs(&scheme, "profile-c", []string{"--lang=en-US"})
	if ExtractFingerprintSeed(other) == ExtractFingerprintSeed(first) {
		t.Fatalf("不同实例不应得到同一个种子")
	}
	if !hasArgPrefix(first, "--fingerprint-hardware-concurrency=") {
		t.Fatalf("应补齐核心数：%v", first)
	}
}

func TestNewSchemeProfileArgsProducesDifferentDevices(t *testing.T) {
	t.Parallel()
	scheme := BuiltinFingerprintSchemes()[0]
	seeds := map[string]bool{}
	for i := 0; i < 20; i++ {
		args := NewSchemeProfileArgs(&scheme)
		seed := ExtractFingerprintSeed(args)
		if seed == "" || seeds[seed] {
			t.Fatalf("第 %d 次生成的种子为空或重复：%v", i, args)
		}
		seeds[seed] = true
		if !hasArgPrefix(args, "--fingerprint-hardware-concurrency=") || !hasArgPrefix(args, "--timezone=america/los_angeles") {
			t.Fatalf("生成参数不完整：%v", args)
		}
	}
}

func TestSchemeFixedArgsRealCanvasVariant(t *testing.T) {
	t.Parallel()
	var real FingerprintScheme
	for _, s := range BuiltinFingerprintSchemes() {
		if s.SchemeId == "builtin-us-west-real-canvas" {
			real = s
		}
	}
	joined := strings.Join(SchemeFixedArgs(real.Fixed), " ")
	if !strings.Contains(joined, "--disable-spoofing=canvas,gpu") || strings.Contains(joined, "WebGPUService") {
		t.Fatalf("真实画布方案参数不对：%s", joined)
	}
}

func TestNormalizeSchemeInputRejectsBadValues(t *testing.T) {
	t.Parallel()
	_, err := NormalizeSchemeInput(FingerprintSchemeInput{SchemeName: "x", Fixed: FingerprintSchemeFixed{Brand: "Safari"}})
	if err == nil {
		t.Fatalf("Safari 品牌应被拒绝")
	}
	_, err = NormalizeSchemeInput(FingerprintSchemeInput{SchemeName: "x", Device: FingerprintSchemeDevice{Memory: []WeightedValue{{Value: "16", Weight: 1}}}})
	if err == nil {
		t.Fatalf("内存 16 应被拒绝（浏览器上限 8）")
	}
	ok, err := NormalizeSchemeInput(FingerprintSchemeInput{SchemeName: " 方案 ", Fixed: FingerprintSchemeFixed{Brand: "edge", Platform: "mac"}})
	if err != nil || ok.Fixed.Brand != "Edge" || ok.Fixed.Platform != "macos" || ok.SchemeName != "方案" {
		t.Fatalf("整理结果不对：%+v err=%v", ok, err)
	}
}