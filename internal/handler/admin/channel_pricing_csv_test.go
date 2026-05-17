package adminhandler

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"tracehub/internal/pkg/config"
)

func TestParseChannelPricingBatchCSVBytes(t *testing.T) {
	ctx := context.Background()
	body := []byte(`channel_id,model_name,enabled,quota_in_per_1k,quota_out_per_1k,input_rate,output_rate
1,gpt-4o,true,100,200,1,1
2,claude-3,false,,,1,2
`)
	items, errMsg := parseChannelPricingBatchCSVBytes(ctx, nil, body, nil)
	if errMsg != "" {
		t.Fatalf("unexpected err: %s", errMsg)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items want 2", len(items))
	}
	if items[0].ChannelID != 1 || items[0].ModelName != "gpt-4o" || !items[0].Enabled {
		t.Fatalf("row0: %+v", items[0])
	}
	if items[1].ChannelID != 2 || items[1].ModelName != "claude-3" || items[1].Enabled {
		t.Fatalf("row1: %+v", items[1])
	}
}

func TestParseChannelPricingBatchCSVBytes_missingChannelID(t *testing.T) {
	ctx := context.Background()
	body := []byte(`model_name,enabled,quota_in_per_1k,quota_out_per_1k,input_rate,output_rate
gpt-4o,true,,,1,1
`)
	_, errMsg := parseChannelPricingBatchCSVBytes(ctx, nil, body, nil)
	if errMsg == "" || !strings.Contains(errMsg, "channel_id") {
		t.Fatalf("want channel_id error, got %q", errMsg)
	}
}

func TestParseChannelPricingCSVBytes_fixedChannel(t *testing.T) {
	ctx := context.Background()
	body := []byte(`model_name,enabled,quota_in_per_1k,quota_out_per_1k,input_rate,output_rate
gpt-4o-mini,true,50,50,1,1
`)
	items, errMsg := parseChannelPricingCSVBytes(ctx, nil, body, nil, 7)
	if errMsg != "" {
		t.Fatalf("unexpected err: %s", errMsg)
	}
	if len(items) != 1 || items[0].ChannelID != 7 {
		t.Fatalf("got %+v", items)
	}
}

func TestGlobalItemsToChannelItems(t *testing.T) {
	q := int64(10)
	global := []globalPricingUpsertItem{{
		ModelName: "m", Enabled: true, QuotaInPer1k: &q, InputRate: 1, OutputRate: 2,
	}}
	out := globalItemsToChannelItems(global, 3)
	if len(out) != 1 || out[0].ChannelID != 3 || out[0].ModelName != "m" {
		t.Fatalf("got %+v", out)
	}
}

func TestParseChannelPricingBatchCSVBytes_tooManyRows(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	buf.WriteString("channel_id,model_name,enabled,quota_in_per_1k,quota_out_per_1k,input_rate,output_rate\n")
	for i := 0; i <= channelPricingCSVMaxRows; i++ {
		buf.WriteString("1,m")
		buf.WriteString(",true,,,1,1\n")
	}
	_, errMsg := parseChannelPricingBatchCSVBytes(ctx, nil, buf.Bytes(), &config.Config{})
	if errMsg == "" || !strings.Contains(errMsg, "5000") {
		t.Fatalf("want row limit error, got %q", errMsg)
	}
}
