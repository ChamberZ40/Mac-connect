package feishu

import "testing"

// A secret spelled out in the command must be hidden; a *reference* to one must
// not be, because redacting `$(cmd …)` hides nothing and swallows the rest of
// the line — that is what turned `Bearer $(lark-cli --profile x token)` into
// `Bearer [redacted] --profile x token)`.
func TestRedactInlineSecrets(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "literal bearer token is hidden",
			in:   `curl -H "Authorization: Bearer abc123secretvalue" https://api.example.com`,
			want: `curl -H "Authorization: Bearer [redacted]" https://api.example.com`,
		},
		{
			name: "command substitution is kept",
			in:   `curl -H "Authorization: Bearer $(lark-cli --profile dev token)" https://open.feishu.cn`,
			want: `curl -H "Authorization: Bearer $(lark-cli --profile dev token)" https://open.feishu.cn`,
		},
		{
			name: "backtick substitution is kept",
			in:   "curl -H \"Authorization: Bearer `get-token`\" https://x",
			want: "curl -H \"Authorization: Bearer `get-token`\" https://x",
		},
		{
			name: "variable expansion is kept",
			in:   `curl -H "Authorization: Bearer $TOKEN" https://x`,
			want: `curl -H "Authorization: Bearer $TOKEN" https://x`,
		},
		{
			name: "literal assignment is hidden",
			in:   `TOKEN=t-abc123 ./run.sh`,
			want: `TOKEN=[redacted] ./run.sh`,
		},
		{
			name: "assignment from substitution is kept",
			in:   `TOKEN=$(lark-cli --profile dev-feishu --as bot token 2>/dev/null)`,
			want: `TOKEN=$(lark-cli --profile dev-feishu --as bot token 2>/dev/null)`,
		},
		{
			name: "assignment from another variable is kept",
			in:   `API_KEY=${OPENAI_API_KEY} python3 run.py`,
			want: `API_KEY=${OPENAI_API_KEY} python3 run.py`,
		},
		{
			name: "quoted literal is still hidden",
			in:   `APP_SECRET="s-9f8e7d6c" make deploy`,
			want: `APP_SECRET=[redacted] make deploy`,
		},
		{
			name: "quoted substitution is kept",
			in:   `APP_SECRET="$(cat secret.txt)" make deploy`,
			want: `APP_SECRET="$(cat secret.txt)" make deploy`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := redactInlineSecrets(tc.in); got != tc.want {
				t.Fatalf("redactInlineSecrets(%q)\n got  %q\n want %q", tc.in, got, tc.want)
			}
		})
	}
}
