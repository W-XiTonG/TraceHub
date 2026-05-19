package billing

// ModelBillRatesTestOpts 供 gateway 等跨包测试构造 ModelBillRates（字段与 Engine 产出一致）。
type ModelBillRatesTestOpts struct {
	Billable      bool
	Mode          string
	PerRequest    int64
	InPer1k       int64
	OutPer1k      int64
	InRate        float64
	OutRate       float64
	GroupMul      float64
	UserMul       float64
	MarkupPercent float64
}

// NewModelBillRatesForTest 构造 ModelBillRates；仅用于测试，勿在生产路径调用。
func NewModelBillRatesForTest(o ModelBillRatesTestOpts) ModelBillRates {
	return ModelBillRates{
		Billable:      o.Billable,
		billingMode:   o.Mode,
		perRequest:    o.PerRequest,
		in1k:          o.InPer1k,
		out1k:         o.OutPer1k,
		inRate:        o.InRate,
		outRate:       o.OutRate,
		groupMul:      o.GroupMul,
		userMul:       o.UserMul,
		markupPercent: o.MarkupPercent,
	}
}
