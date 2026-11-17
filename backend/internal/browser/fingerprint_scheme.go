package browser

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
)

// FingerprintSchemeNone 表示实例明确不使用任何指纹方案（手动配置）
const FingerprintSchemeNone = "__none__"

// WeightedValue 设备参数池里的一个候选值及其权重
type WeightedValue struct {
	Value  string `json:"value"`
	Weight int    `json:"weight"`
}

// FingerprintSchemeFixed 方案固定项：同一方案下所有窗口保持一致的部分
type FingerprintSchemeFixed struct {
	Platform     string `json:"platform"`     // windows / macos / linux
	Brand        string `json:"brand"`        // Chrome / Edge / Opera / Vivaldi
	Lang         string `json:"lang"`         // en-US
	Timezone     string `json:"timezone"`     // America/Los_Angeles
	WebrtcPolicy string `json:"webrtcPolicy"` // disable_non_proxied_udp
	CanvasNoise  bool   `json:"canvasNoise"`  // true=按种子加噪声；false=--disable-spoofing=canvas
	GpuSpoof     bool   `json:"gpuSpoof"`     // true=按种子换显卡型号；false=--disable-spoofing=gpu
	HideWebGPU   bool   `json:"hideWebGPU"`   // true=--disable-features=WebGPUService（避免 WebGPU 暴露真实显卡）
	SpeechAPI    string `json:"speechApi"`    // disable / enable
	CoreId       string `json:"coreId"`       // 可选：方案指定内核
	Maximized    bool   `json:"maximized"`    // true=--start-maximized，不再写死窗口尺寸
}

// FingerprintSchemeDevice 设备池：每个新窗口从中随机抽取，保证同方案不同窗口是不同设备
type FingerprintSchemeDevice struct {
	Cores            []WeightedValue `json:"cores"`
	Memory           []WeightedValue `json:"memory"`
	PlatformVersions []WeightedValue `json:"platformVersions"`
	WindowSizes      []WeightedValue `json:"windowSizes"`
}

// FingerprintScheme 指纹方案
type FingerprintScheme struct {
	SchemeId    string                  `json:"schemeId"`
	SchemeName  string                  `json:"schemeName"`
	Description string                  `json:"description"`
	Builtin     bool                    `json:"builtin"`
	IsDefault   bool                    `json:"isDefault"`
	SortOrder   int                     `json:"sortOrder"`
	Fixed       FingerprintSchemeFixed  `json:"fixed"`
	Device      FingerprintSchemeDevice `json:"device"`
	CreatedAt   string                  `json:"createdAt"`
	UpdatedAt   string                  `json:"updatedAt"`
}

// FingerprintSchemeInput 新建/修改方案的输入
type FingerprintSchemeInput struct {
	SchemeName  string                  `json:"schemeName"`
	Description string                  `json:"description"`
	Fixed       FingerprintSchemeFixed  `json:"fixed"`
	Device      FingerprintSchemeDevice `json:"device"`
}

// 方案固定项覆盖的参数前缀：应用方案时先从实例参数里剔除这些，再按方案重新写入
var schemeFixedArgPrefixes = []string{
	"--fingerprint-platform=", "--fingerprint-brand=", "--lang=", "--accept-lang=", "--accept-language=",
	"--timezone=", "--webrtc-ip-handling-policy=", "--disable-spoofing=", "--disable-features=",
	"--disable-speech-api", "--enable-speech-api", "--start-maximized",
	// 历史上写过、但内核并不识别的无效开关，一并清理
	"--fingerprint-color-depth=", "--fingerprint-canvas-noise=", "--fingerprint-audio-noise=",
	"--fingerprint-do-not-track=", "--fingerprint-fonts=", "--fingerprint-media-devices=", "--fingerprint-touch-points=",
	"--fingerprint-webgl-vendor=", "--fingerprint-webgl-renderer=", "--fingerprint-gpu-vendor=", "--fingerprint-gpu-renderer=",
	"--fingerprint-screen-width=", "--fingerprint-screen-height=", "--fingerprint-device-scale-factor=",
}

func defaultWindowsDevicePool() FingerprintSchemeDevice {
	return FingerprintSchemeDevice{
		Cores: []WeightedValue{{"4", 15}, {"6", 20}, {"8", 35}, {"12", 15}, {"16", 10}, {"20", 3}, {"24", 2}},
		Memory: []WeightedValue{{"8", 100}},
		PlatformVersions: []WeightedValue{{"10.0.0", 30}, {"15.0.0", 30}, {"19.0.0", 40}},
		WindowSizes: []WeightedValue{{"1920,1080", 40}, {"1536,864", 20}, {"1366,768", 15}, {"1440,900", 10}, {"1600,900", 10}, {"2560,1440", 5}},
	}
}

func usFixed(timezone string) FingerprintSchemeFixed {
	return FingerprintSchemeFixed{
		Platform: "windows", Brand: "Chrome", Lang: "en-US", Timezone: timezone,
		WebrtcPolicy: "disable_non_proxied_udp", CanvasNoise: true, GpuSpoof: true, HideWebGPU: true,
		SpeechAPI: "disable", CoreId: "core-Chrom-144", Maximized: true,
	}
}

// BuiltinFingerprintSchemes 内置默认方案（第一次启动写入数据库，之后可在界面上改）
func BuiltinFingerprintSchemes() []FingerprintScheme {
	laptopPool := defaultWindowsDevicePool()
	laptopPool.Cores = []WeightedValue{{"4", 20}, {"8", 40}, {"12", 25}, {"16", 10}, {"20", 5}}
	laptopPool.WindowSizes = []WeightedValue{{"1536,864", 35}, {"1366,768", 25}, {"1920,1080", 25}, {"1280,720", 10}, {"1440,900", 5}}

	edgeFixed := usFixed("America/Los_Angeles")
	edgeFixed.Brand = "Edge"

	realFixed := usFixed("America/Los_Angeles")
	realFixed.CanvasNoise = false
	realFixed.GpuSpoof = false
	realFixed.HideWebGPU = false

	return []FingerprintScheme{
		{SchemeId: "builtin-us-west-desktop", SchemeName: "美西洛杉矶 · Windows 台式机（默认）", Description: "匹配洛杉矶出口的梯子：en-US + America/Los_Angeles + WebRTC 禁用非代理 UDP + 屏蔽系统语音；每个窗口按种子随机显卡/画布/音频，核心数与窗口尺寸从台式机池里抽取。", Builtin: true, IsDefault: true, SortOrder: 1, Fixed: usFixed("America/Los_Angeles"), Device: defaultWindowsDevicePool()},
		{SchemeId: "builtin-us-west-laptop", SchemeName: "美西洛杉矶 · Windows 笔记本", Description: "同默认方案的地区设置，核心数与窗口尺寸改为笔记本常见分布（8 核、1536x864 为主）。", Builtin: true, SortOrder: 2, Fixed: usFixed("America/Los_Angeles"), Device: laptopPool},
		{SchemeId: "builtin-us-west-edge", SchemeName: "美西洛杉矶 · Microsoft Edge", Description: "品牌改为 Edge（UA 带 Edg/ 版本号），其余与默认方案一致，用于分散浏览器品牌。", Builtin: true, SortOrder: 3, Fixed: edgeFixed, Device: defaultWindowsDevicePool()},
		{SchemeId: "builtin-us-west-real-canvas", SchemeName: "美西洛杉矶 · 单号高风控（真实画布）", Description: "关闭画布像素篡改与显卡型号伪装、保留 WebGPU，画布字宽恢复正常；适合 PayPal、TikTok、Amazon 等强风控单号。注意：同一台电脑上用此方案的窗口画布指纹相同。", Builtin: true, SortOrder: 4, Fixed: realFixed, Device: defaultWindowsDevicePool()},
		{SchemeId: "builtin-us-east", SchemeName: "美东纽约 · Windows（出口在美东时用）", Description: "America/New_York；只有当代理出口 IP 在美东时才选它，否则时区会和 IP 对不上。", Builtin: true, SortOrder: 5, Fixed: usFixed("America/New_York"), Device: defaultWindowsDevicePool()},
		{SchemeId: "builtin-us-central", SchemeName: "美中芝加哥 · Windows（出口在美中时用）", Description: "America/Chicago；只有当代理出口 IP 在美中时才选它。", Builtin: true, SortOrder: 6, Fixed: usFixed("America/Chicago"), Device: defaultWindowsDevicePool()},
	}
}

func pickWeighted(pool []WeightedValue, rnd func(n int) int) string {
	total := 0
	for _, item := range pool {
		if item.Weight > 0 && strings.TrimSpace(item.Value) != "" {
			total += item.Weight
		}
	}
	if total <= 0 {
		return ""
	}
	target := rnd(total)
	for _, item := range pool {
		if item.Weight <= 0 || strings.TrimSpace(item.Value) == "" {
			continue
		}
		target -= item.Weight
		if target < 0 {
			return strings.TrimSpace(item.Value)
		}
	}
	return strings.TrimSpace(pool[len(pool)-1].Value)
}

func cryptoIntn(n int) int {
	if n <= 1 {
		return 0
	}
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0
	}
	return int(binary.BigEndian.Uint64(buf[:]) % uint64(n))
}

// deterministicRandom 由稳定字符串派生的伪随机抽取器（同一实例每次得到同样的设备参数）
func deterministicRandom(key string) func(n int) int {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	state := h.Sum64()
	return func(n int) int {
		if n <= 1 {
			return 0
		}
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		return int(state % uint64(n))
	}
}

// LegacyFallbackSeed 与旧版 buildBrowserLaunchArgs 一致的回退种子（实例没写 --fingerprint= 时内核实际收到的值）
func LegacyFallbackSeed(profileId string) string {
	seed := 0
	for _, char := range profileId {
		seed = (seed << 5) - seed + int(char)
	}
	if seed < 0 {
		seed = -seed
	}
	return strconv.Itoa(seed)
}

// SchemeFixedArgs 方案固定项对应的启动参数
func SchemeFixedArgs(fixed FingerprintSchemeFixed) []string {
	out := make([]string, 0, 10)
	platform := strings.ToLower(strings.TrimSpace(fixed.Platform))
	if platform == "mac" {
		platform = "macos"
	}
	if platform != "" {
		out = append(out, "--fingerprint-platform="+platform)
	}
	brand := strings.TrimSpace(fixed.Brand)
	if brand == "" {
		brand = "Chrome"
	}
	out = append(out, "--fingerprint-brand="+brand)
	if lang := strings.TrimSpace(fixed.Lang); lang != "" {
		out = append(out, "--lang="+lang)
	}
	if tz := strings.TrimSpace(fixed.Timezone); tz != "" {
		out = append(out, "--timezone="+tz)
	}
	if policy := strings.TrimSpace(fixed.WebrtcPolicy); policy != "" {
		out = append(out, "--webrtc-ip-handling-policy="+policy)
	}
	disabled := make([]string, 0, 2)
	if !fixed.CanvasNoise {
		disabled = append(disabled, "canvas")
	}
	if !fixed.GpuSpoof {
		disabled = append(disabled, "gpu")
	}
	if len(disabled) > 0 {
		out = append(out, "--disable-spoofing="+strings.Join(disabled, ","))
	}
	if fixed.HideWebGPU {
		out = append(out, "--disable-features=WebGPUService")
	}
	if strings.EqualFold(strings.TrimSpace(fixed.SpeechAPI), "enable") {
		out = append(out, "--enable-speech-api")
	} else {
		out = append(out, "--disable-speech-api")
	}
	if fixed.Maximized {
		out = append(out, "--start-maximized")
	}
	return out
}

// SchemeDeviceArgs 从设备池抽取一套设备参数（rnd 为空时用加密随机数）
func SchemeDeviceArgs(device FingerprintSchemeDevice, maximized bool, rnd func(n int) int) []string {
	if rnd == nil {
		rnd = cryptoIntn
	}
	out := []string{"--fingerprint=" + fmt.Sprintf("%d", rnd(2147483646)+1)}
	if v := pickWeighted(device.Cores, rnd); v != "" {
		out = append(out, "--fingerprint-hardware-concurrency="+v)
	}
	if v := pickWeighted(device.Memory, rnd); v != "" {
		out = append(out, "--fingerprint-memory="+v)
	}
	if v := pickWeighted(device.PlatformVersions, rnd); v != "" {
		out = append(out, "--fingerprint-platform-version="+v)
	}
	if !maximized {
		if v := pickWeighted(device.WindowSizes, rnd); v != "" {
			out = append(out, "--window-size="+v)
		}
	}
	return out
}

func argMatchesPrefix(arg string, prefixes []string) bool {
	lower := strings.ToLower(strings.TrimSpace(arg))
	for _, prefix := range prefixes {
		if strings.HasSuffix(prefix, "=") {
			if strings.HasPrefix(lower, prefix) {
				return true
			}
		} else if lower == prefix {
			return true
		}
	}
	return false
}

func hasArgPrefix(args []string, prefix string) bool {
	for _, arg := range args {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(arg)), prefix) {
			return true
		}
	}
	return false
}

// ApplySchemeToArgs 把方案套在实例现有参数上：固定项以方案为准，设备项保留实例原值；
// 缺少的设备项按实例 ID 稳定派生，保证同一实例每次启动一致、不同实例互不相同。
func ApplySchemeToArgs(scheme *FingerprintScheme, profileId string, existing []string) []string {
	if scheme == nil {
		return append([]string{}, existing...)
	}
	kept := make([]string, 0, len(existing))
	for _, arg := range existing {
		trimmed := strings.TrimSpace(arg)
		if trimmed == "" || argMatchesPrefix(trimmed, schemeFixedArgPrefixes) {
			continue
		}
		if scheme.Fixed.Maximized && strings.HasPrefix(strings.ToLower(trimmed), "--window-size=") {
			continue
		}
		kept = append(kept, trimmed)
	}
	if !hasArgPrefix(kept, "--fingerprint=") {
		kept = append([]string{"--fingerprint=" + LegacyFallbackSeed(profileId)}, kept...)
	}
	rnd := deterministicRandom(profileId + "|" + scheme.SchemeId)
	if !hasArgPrefix(kept, "--fingerprint-hardware-concurrency=") {
		if v := pickWeighted(scheme.Device.Cores, rnd); v != "" {
			kept = append(kept, "--fingerprint-hardware-concurrency="+v)
		}
	}
	if !hasArgPrefix(kept, "--fingerprint-memory=") && !hasArgPrefix(kept, "--fingerprint-device-memory=") {
		if v := pickWeighted(scheme.Device.Memory, rnd); v != "" {
			kept = append(kept, "--fingerprint-memory="+v)
		}
	}
	if !scheme.Fixed.Maximized && !hasArgPrefix(kept, "--window-size=") {
		if v := pickWeighted(scheme.Device.WindowSizes, rnd); v != "" {
			kept = append(kept, "--window-size="+v)
		}
	}
	return append(kept, SchemeFixedArgs(scheme.Fixed)...)
}

// NewSchemeProfileArgs 为新窗口生成一整套参数：固定项 + 全新随机设备
func NewSchemeProfileArgs(scheme *FingerprintScheme) []string {
	if scheme == nil {
		return nil
	}
	return append(SchemeDeviceArgs(scheme.Device, scheme.Fixed.Maximized, nil), SchemeFixedArgs(scheme.Fixed)...)
}

// ExtractFingerprintSeed 读取参数里的种子
func ExtractFingerprintSeed(args []string) string {
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		if strings.HasPrefix(strings.ToLower(trimmed), "--fingerprint=") {
			return strings.TrimSpace(trimmed[len("--fingerprint="):])
		}
	}
	return ""
}

// ReplaceFingerprintSeed 替换（或补上）种子
func ReplaceFingerprintSeed(args []string, seed string) []string {
	seedArg := "--fingerprint=" + seed
	out := make([]string, 0, len(args)+1)
	replaced := false
	for _, arg := range args {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(arg)), "--fingerprint=") {
			if !replaced {
				out = append(out, seedArg)
				replaced = true
			}
			continue
		}
		out = append(out, arg)
	}
	if !replaced {
		out = append([]string{seedArg}, out...)
	}
	return out
}

func normalizeWeightedPool(pool []WeightedValue) []WeightedValue {
	out := make([]WeightedValue, 0, len(pool))
	for _, item := range pool {
		value := strings.TrimSpace(item.Value)
		if value == "" {
			continue
		}
		weight := item.Weight
		if weight <= 0 {
			weight = 1
		}
		out = append(out, WeightedValue{Value: value, Weight: weight})
	}
	return out
}

// NormalizeSchemeInput 整理输入并补默认值
func NormalizeSchemeInput(input FingerprintSchemeInput) (FingerprintSchemeInput, error) {
	input.SchemeName = strings.TrimSpace(input.SchemeName)
	if input.SchemeName == "" {
		return input, fmt.Errorf("方案名称不能为空")
	}
	input.Description = strings.TrimSpace(input.Description)
	f := &input.Fixed
	f.Platform = strings.ToLower(strings.TrimSpace(f.Platform))
	if f.Platform == "mac" {
		f.Platform = "macos"
	}
	switch f.Platform {
	case "", "windows", "macos", "linux":
	default:
		return input, fmt.Errorf("不支持的操作系统：%s", f.Platform)
	}
	switch strings.ToLower(strings.TrimSpace(f.Brand)) {
	case "", "chrome":
		f.Brand = "Chrome"
	case "edge":
		f.Brand = "Edge"
	case "opera":
		f.Brand = "Opera"
	case "vivaldi":
		f.Brand = "Vivaldi"
	default:
		return input, fmt.Errorf("内核只支持 Chrome / Edge / Opera / Vivaldi 品牌：%s", f.Brand)
	}
	f.Lang = strings.TrimSpace(f.Lang)
	f.Timezone = strings.TrimSpace(f.Timezone)
	f.WebrtcPolicy = strings.TrimSpace(f.WebrtcPolicy)
	if f.SpeechAPI != "enable" {
		f.SpeechAPI = "disable"
	}
	f.CoreId = strings.TrimSpace(f.CoreId)
	d := &input.Device
	d.Cores = normalizeWeightedPool(d.Cores)
	d.Memory = normalizeWeightedPool(d.Memory)
	d.PlatformVersions = normalizeWeightedPool(d.PlatformVersions)
	d.WindowSizes = normalizeWeightedPool(d.WindowSizes)
	for _, item := range d.Cores {
		if n, err := strconv.Atoi(item.Value); err != nil || n < 1 || n > 128 {
			return input, fmt.Errorf("CPU 核心数池含无效值：%s", item.Value)
		}
	}
	for _, item := range d.Memory {
		if n, err := strconv.Atoi(item.Value); err != nil || n < 1 || n > 8 {
			return input, fmt.Errorf("设备内存池只能填 1～8（浏览器上限 8 GB）：%s", item.Value)
		}
	}
	for _, item := range d.WindowSizes {
		if _, _, ok := parseResolutionPairForScheme(item.Value); !ok {
			return input, fmt.Errorf("窗口尺寸池含无效值：%s（示例 1920,1080）", item.Value)
		}
	}
	return input, nil
}

func parseResolutionPairForScheme(raw string) (int, int, bool) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	sep := ","
	if !strings.Contains(raw, ",") && strings.Contains(raw, "x") {
		sep = "x"
	}
	parts := strings.Split(raw, sep)
	if len(parts) != 2 {
		return 0, 0, false
	}
	w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || w < 320 || h < 240 {
		return 0, 0, false
	}
	return w, h, true
}

func sortSchemes(list []*FingerprintScheme) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].IsDefault != list[j].IsDefault {
			return list[i].IsDefault
		}
		if list[i].SortOrder != list[j].SortOrder {
			return list[i].SortOrder < list[j].SortOrder
		}
		return list[i].CreatedAt < list[j].CreatedAt
	})
}
