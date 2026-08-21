package nodeseek

import "testing"

// TestParseUserStats 用真实 getInfo 响应样例校验解析。
func TestParseUserStats(t *testing.T) {
	body := []byte(`{"success":true,"detail":{"member_id":9037,"member_name":"idamie","isAdmin":0,"rank":1,"coin":279,"stardust":0,"bio":null,"created_at":"2023-11-12T03:35:23.000Z","nPost":1,"nComment":31,"follows":0,"fans":0,"created_at_str":"1013days ago","roles":[],"readme":null,"followed":false}}`)
	s, err := parseUserStats(body)
	if err != nil {
		t.Fatalf("parseUserStats error: %v", err)
	}
	if s.Rank != 1 || s.Chicken != 279 || s.Topics != 1 || s.Comments != 31 {
		t.Fatalf("unexpected stats: %+v", s)
	}
	if s.JoinDays <= 0 {
		t.Fatalf("join_days should be > 0, got %d", s.JoinDays)
	}
	t.Logf("stats=%+v", s)
}

// TestParseUserStatsFail 校验 success=false 返回错误（fail-closed）。
func TestParseUserStatsFail(t *testing.T) {
	if _, err := parseUserStats([]byte(`{"success":false,"detail":null}`)); err == nil {
		t.Fatal("expected error for success=false")
	}
}

// TestParseMessageList 用真实 message/list 响应样例校验命中。
func TestParseMessageList(t *testing.T) {
	body := []byte(`{"success":true,"msgArray":[{"receiver_id":9037,"sender_id":37384,"max_id":7604259,"content":"NS_AUTH_4388AE11","created_at":"2026-08-21T05:06:55.000Z","viewed":0,"sender_name":"萧炎","receiver_name":"idamie"}]}`)
	ok, err := parseMessageList(body, "37384", "NS_AUTH_4388AE11")
	if err != nil {
		t.Fatalf("parseMessageList error: %v", err)
	}
	if !ok {
		t.Fatal("expected hit for sender 37384 + code NS_AUTH_4388AE11")
	}
	// 错误 code → 未收到。
	if ok2, _ := parseMessageList(body, "37384", "WRONG"); ok2 {
		t.Fatal("expected miss for wrong code")
	}
	// 错误 sender → 未收到。
	if ok3, _ := parseMessageList(body, "99999", "NS_AUTH_4388AE11"); ok3 {
		t.Fatal("expected miss for wrong sender")
	}
}

// TestDecodePjwtPayload 用真实 pjwt 载荷样例校验解析。
func TestDecodePjwtPayload(t *testing.T) {
	id, name, err := decodePjwtPayload("eyJpZCI6MzczODQsIm5hbWUiOiLokKfngo4iLCJ0cyI6MTc4NjYxOTM4M30")
	if err != nil {
		t.Fatalf("decodePjwtPayload error: %v", err)
	}
	if id != "37384" || name != "萧炎" {
		t.Fatalf("unexpected id/name: %q / %q", id, name)
	}
}

// TestParsePjwt 校验从 Cookie 串提取 pjwt（含 3 段 JWT、NS 实际 2 段格式与裸 payload 三种）。
func TestParsePjwt(t *testing.T) {
	// 完整 JWT（header.payload.signature）。
	id, name, err := parsePjwt("pjwt=eyJhbGciOiJub25lIn0.eyJpZCI6MzczODQsIm5hbWUiOiLokKfngo4iLCJ0cyI6MTc4NjYxOTM4M30.sig")
	if err != nil {
		t.Fatalf("parsePjwt(jwt) error: %v", err)
	}
	if id != "37384" || name != "萧炎" {
		t.Fatalf("unexpected: %q / %q", id, name)
	}
	// NS 实际 2 段格式（payload.signature）——真实 Cookie 样例，回归：payload 必须取倒数第二段。
	id2, name2, err := parsePjwt("pjwt=eyJpZCI6MzczODQsIm5hbWUiOiLokKfngo4iLCJ0cyI6MTc4NjYxOTM4M30.0TPcLa3TF6cLVFzwVuqxGS5js7p6p8k3ixsLzTcGxLY")
	if err != nil {
		t.Fatalf("parsePjwt(2seg) error: %v", err)
	}
	if id2 != "37384" || name2 != "萧炎" {
		t.Fatalf("unexpected 2seg: %q / %q", id2, name2)
	}
	// 缺少 pjwt → 错误。
	if _, _, err := parsePjwt("other=1"); err == nil {
		t.Fatal("expected error for missing pjwt")
	}
}
