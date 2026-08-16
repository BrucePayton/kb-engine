package feishu

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestFeishuImageDownloadTimeout(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "unset uses default", want: 30 * time.Second},
		{name: "custom seconds", value: "120", want: 120 * time.Second},
		{name: "whitespace is accepted", value: " 45 ", want: 45 * time.Second},
		{name: "invalid uses default", value: "invalid", want: 30 * time.Second},
		{name: "zero uses default", value: "0", want: 30 * time.Second},
		{name: "negative uses default", value: "-1", want: 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("FEISHU_IMAGE_DOWNLOAD_TIMEOUT_SECONDS", tt.value)
			if got := feishuImageDownloadTimeout(); got != tt.want {
				t.Errorf("feishuImageDownloadTimeout() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestNewFeishuImageDownloadClientUsesConfiguredTimeout(t *testing.T) {
	t.Setenv("FEISHU_IMAGE_DOWNLOAD_TIMEOUT_SECONDS", "120")

	if got := newFeishuImageDownloadClient().Timeout; got != 120*time.Second {
		t.Errorf("newFeishuImageDownloadClient().Timeout = %s, want %s", got, 120*time.Second)
	}
}

func TestFeishuImageSizeLimits(t *testing.T) {
	const want = 20 << 20

	if feishuMaxImageBytes != want {
		t.Errorf("feishuMaxImageBytes = %d, want %d", feishuMaxImageBytes, want)
	}
}

func TestBuildStreamingCardJSONUsesProductTitle(t *testing.T) {
	var card struct {
		Header struct {
			Title struct {
				Content string `json:"content"`
			} `json:"title"`
		} `json:"header"`
	}
	if err := json.Unmarshal([]byte(buildStreamingCardJSON(RegionFeishu)), &card); err != nil {
		t.Fatalf("unmarshal streaming card: %v", err)
	}
	if got, want := card.Header.Title.Content, "WeKnora"; got != want {
		t.Errorf("streaming card title = %q, want %q", got, want)
	}
}

func TestFeishuThreadID_ThreadedReply(t *testing.T) {
	// Simulate: message is a reply in a thread (root_id is set)
	msg := &feishuMessage{
		MessageID: "msg-reply-1",
		RootID:    "msg-root-1",
		ParentID:  "msg-parent-1",
	}

	threadID := msg.RootID
	if threadID == "" {
		threadID = msg.MessageID
	}

	if threadID != "msg-root-1" {
		t.Errorf("threadID = %q, want %q", threadID, "msg-root-1")
	}
}

func TestFeishuThreadID_TopLevelMessage(t *testing.T) {
	// Simulate: top-level message (root_id is empty)
	msg := &feishuMessage{
		MessageID: "msg-top-1",
		RootID:    "",
		ParentID:  "",
	}

	threadID := msg.RootID
	if threadID == "" {
		threadID = msg.MessageID
	}

	if threadID != "msg-top-1" {
		t.Errorf("threadID = %q, want %q (should use MessageID as fallback)", threadID, "msg-top-1")
	}
}

func TestFeishuMessageStruct_JSONFields(t *testing.T) {
	// Verify the struct fields exist and have correct zero values
	msg := feishuMessage{}
	if msg.RootID != "" {
		t.Errorf("RootID zero value = %q, want empty", msg.RootID)
	}
	if msg.ParentID != "" {
		t.Errorf("ParentID zero value = %q, want empty", msg.ParentID)
	}
	if msg.MessageID != "" {
		t.Errorf("MessageID zero value = %q, want empty", msg.MessageID)
	}
}

func TestImageCacheKey_StripsQuery(t *testing.T) {
	cases := map[string]string{
		"https://host/a.png?sig=1&t=2": "https://host/a.png",
		"https://host/a.png":           "https://host/a.png",
	}
	for in, want := range cases {
		if got := imageCacheKey(in); got != want {
			t.Errorf("imageCacheKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveMarkdownImages_NoImageUnchanged(t *testing.T) {
	a := &Adapter{region: RegionFeishu}
	in := "hello **world** [link](https://example.com)"
	if got := a.resolveMarkdownImages(context.Background(), "tok", in); got != in {
		t.Errorf("content without image was modified: %q", got)
	}
}

func TestResolveMarkdownImages_FallbackToLinkOnFailure(t *testing.T) {
	a := &Adapter{region: RegionFeishu}
	// A direct-IP loopback URL fails SSRF validation before any network call,
	// so the image must degrade to a plain markdown link (never left as ![]()).
	in := "see ![diagram](http://127.0.0.1/x.png) here"
	got := a.resolveMarkdownImages(context.Background(), "tok", in)
	if strings.Contains(got, "![") {
		t.Errorf("failed image should not remain as image markdown: %q", got)
	}
	if !strings.Contains(got, "[diagram](http://127.0.0.1/x.png)") {
		t.Errorf("expected link fallback with alt text, got: %q", got)
	}
}

// An image with no alt text falls back to the region's own label, so Lark users
// do not get a Chinese link label.
func TestResolveMarkdownImages_FallbackLabelFollowsRegion(t *testing.T) {
	for _, region := range []Region{RegionFeishu, RegionLark} {
		a := &Adapter{region: region}
		got := a.resolveMarkdownImages(context.Background(), "tok", "![](http://127.0.0.1/x.png)")
		want := "[" + region.ImageFallbackLabel + "](http://127.0.0.1/x.png)"
		if got != want {
			t.Errorf("%s fallback = %q, want %q", region.Label, got, want)
		}
	}
}
