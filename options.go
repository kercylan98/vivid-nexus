package nexus

// Option 是用于配置 Options 的函数类型。
//
// 通常通过 WithOptions、WithSessionReaderProvider 等构造函数注入；
// 在 New 中通过 NewOptions(opts...) 应用，未应用的字段保持零值或默认值。
type Option func(o *Options)

// NewOptions 根据传入的 Option 列表构造 Options。
//
// 默认将 SessionReaderProvider 设为按字节流读取的默认实现；
// 后续 Option 可覆盖该字段。未通过 Option 设置的字段为零值。
func NewOptions(opts ...Option) *Options {
	options := &Options{
		SessionReaderProvider: SessionReaderProviderFN(newDefaultSessionReader),
	}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

// Options 存放 Nexus 与 session 的配置。
//
// SessionReaderProvider 可选：未设置时使用默认的按字节流读取实现；
// 可通过 WithSessionReaderProvider 覆盖。使用 WithOptions 克隆时，若源 Options 的该字段为 nil，会补回默认实现。
type Options struct {
	SessionReaderProvider SessionReaderProvider
}

// WithOptions 将给定的 Options 整体复制到构建中的 Options。
//
// 用于从已有配置克隆或批量设置。若 options 为 nil 则不修改目标；
// 若 options.SessionReaderProvider 为 nil，会先赋默认实现再复制，避免目标得到 nil Provider。
func WithOptions(options *Options) Option {
	return func(opts *Options) {
		if options == nil {
			return
		}
		if options.SessionReaderProvider == nil {
			options.SessionReaderProvider = SessionReaderProviderFN(newDefaultSessionReader)
		}
		*opts = *options
	}
}

// WithSessionReaderProvider 设置会话数据读取器提供方。
//
// 每个 Session 在 Prelaunch 时通过该 Provider 获取对应的 SessionReader。
// 若 provider 为 nil 则本 Option 不修改 Options（保留原有或默认值）。
func WithSessionReaderProvider(provider SessionReaderProvider) Option {
	return func(o *Options) {
		if provider == nil {
			return
		}
		o.SessionReaderProvider = provider
	}
}
