package agent

import (
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
)

func TestAgentLoopTrustPolicyDenyReason(t *testing.T) {
	cfg := config.DefaultConfig()
	al := &AgentLoop{cfg: cfg}

	cfg.Trust.ApprovalPolicy = config.ApprovalPolicyAdviceOnly
	if got := al.trustPolicyDenyReason("exec"); got == "" {
		t.Fatal("advice_only should block exec")
	}
	if got := al.trustPolicyDenyReason("write_file"); got == "" {
		t.Fatal("advice_only should block write tools")
	}

	cfg.Trust.ApprovalPolicy = config.ApprovalPolicyConfirmWrite
	if got := al.trustPolicyDenyReason("write_file"); got == "" {
		t.Fatal("confirm_write should block write tools")
	}
	if got := al.trustPolicyDenyReason("exec"); got != "" {
		t.Fatalf("confirm_write should not block exec, got %q", got)
	}

	cfg.Trust.ApprovalPolicy = config.ApprovalPolicyConfirmExec
	if got := al.trustPolicyDenyReason("exec"); got == "" {
		t.Fatal("confirm_exec should block exec")
	}
	if got := al.trustPolicyDenyReason("write_file"); got != "" {
		t.Fatalf("confirm_exec should not block write tools, got %q", got)
	}

	t.Run("confirm policies defer to approvers when present", func(t *testing.T) {
		hm := NewHookManager(nil)
		if err := hm.Mount(NamedHook("deny-approval", &denyApprovalHook{})); err != nil {
			t.Fatalf("Mount() error = %v", err)
		}
		al.hooks = hm

		cfg.Trust.ApprovalPolicy = config.ApprovalPolicyConfirmWrite
		if got := al.trustPolicyDenyReason("write_file"); got != "" {
			t.Fatalf("confirm_write should defer to approver, got %q", got)
		}

		cfg.Trust.ApprovalPolicy = config.ApprovalPolicyConfirmExec
		if got := al.trustPolicyDenyReason("exec"); got != "" {
			t.Fatalf("confirm_exec should defer to approver, got %q", got)
		}
	})
}
