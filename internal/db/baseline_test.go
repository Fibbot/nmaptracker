package db

import (
	"errors"
	"testing"
)

func TestNormalizeBaselineDefinition(t *testing.T) {
	t.Run("ip inference", func(t *testing.T) {
		definition, typ, err := normalizeBaselineDefinition("10.20.30.40")
		if err != nil {
			t.Fatalf("normalize baseline ip: %v", err)
		}
		if definition != "10.20.30.40" || typ != "ip" {
			t.Fatalf("unexpected normalized ip: %q %q", definition, typ)
		}
	})

	t.Run("cidr inference and canonicalization", func(t *testing.T) {
		definition, typ, err := normalizeBaselineDefinition("10.20.30.99/24")
		if err != nil {
			t.Fatalf("normalize baseline cidr: %v", err)
		}
		if definition != "10.20.30.0/24" || typ != "cidr" {
			t.Fatalf("unexpected normalized cidr: %q %q", definition, typ)
		}
	})

	t.Run("cidr broader than /16 rejected", func(t *testing.T) {
		_, _, err := normalizeBaselineDefinition("10.20.0.0/15")
		if !errors.Is(err, ErrBaselineCIDRTooBroad) {
			t.Fatalf("expected ErrBaselineCIDRTooBroad, got %v", err)
		}
	})

	t.Run("ipv6 rejected", func(t *testing.T) {
		_, _, err := normalizeBaselineDefinition("2001:db8::1")
		if !errors.Is(err, ErrBaselineIPv6Unsupported) {
			t.Fatalf("expected ErrBaselineIPv6Unsupported, got %v", err)
		}
	})
}

func TestBulkAddExpectedAssetBaselinesDedupes(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	project, err := db.CreateProject("baseline")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	added, items, err := db.BulkAddExpectedAssetBaselines(project.ID, []string{
		"10.0.0.1",
		"10.0.0.0/24",
		"10.0.0.55/24", // same canonical CIDR
		"10.0.0.1",     // duplicate IP
	})
	if err != nil {
		t.Fatalf("bulk add baseline: %v", err)
	}
	if added != 2 {
		t.Fatalf("expected 2 added baselines, got %d", added)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 inserted baseline items, got %d", len(items))
	}

	added, items, err = db.BulkAddExpectedAssetBaselines(project.ID, []string{"10.0.0.1", "10.0.0.0/24"})
	if err != nil {
		t.Fatalf("bulk add duplicate baseline: %v", err)
	}
	if added != 0 || len(items) != 0 {
		t.Fatalf("expected no-op duplicate insert, got added=%d items=%d", added, len(items))
	}
}

func TestEvaluateExpectedAssetBaselineSetDiffs(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	project, err := db.CreateProject("baseline-eval")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	_, _, err = db.BulkAddExpectedAssetBaselines(project.ID, []string{"10.0.0.0/30"})
	if err != nil {
		t.Fatalf("seed baseline definitions: %v", err)
	}

	_, err = db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.0.0.1", Hostname: "inside", InScope: true})
	if err != nil {
		t.Fatalf("insert host 10.0.0.1: %v", err)
	}
	_, err = db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.0.0.4", Hostname: "outside-a", InScope: false})
	if err != nil {
		t.Fatalf("insert host 10.0.0.4: %v", err)
	}
	_, err = db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.0.0.5", Hostname: "outside-b", InScope: true})
	if err != nil {
		t.Fatalf("insert host 10.0.0.5: %v", err)
	}

	result, err := db.EvaluateExpectedAssetBaseline(project.ID)
	if err != nil {
		t.Fatalf("evaluate baseline: %v", err)
	}

	if result.Summary.ExpectedTotal != 4 {
		t.Fatalf("expected expected_total=4, got %d", result.Summary.ExpectedTotal)
	}
	if result.Summary.ObservedTotal != 3 {
		t.Fatalf("expected observed_total=3, got %d", result.Summary.ObservedTotal)
	}
	if result.Summary.ExpectedButUnseen != 3 {
		t.Fatalf("expected expected_but_unseen=3, got %d", result.Summary.ExpectedButUnseen)
	}
	if result.Summary.SeenButOutOfScope != 2 {
		t.Fatalf("expected seen_but_out_of_scope=2, got %d", result.Summary.SeenButOutOfScope)
	}
	if result.Summary.SeenButOutOfScopeAndMarkedInScope != 1 {
		t.Fatalf("expected seen_but_out_of_scope_and_marked_in_scope=1, got %d", result.Summary.SeenButOutOfScopeAndMarkedInScope)
	}
	if result.Summary.SeenButOutOfScopeAndMarkedOutOfScope != 1 {
		t.Fatalf("expected seen_but_out_of_scope_and_marked_out_scope=1, got %d", result.Summary.SeenButOutOfScopeAndMarkedOutOfScope)
	}

	unseen := result.Lists.ExpectedButUnseen
	if len(unseen) != 3 || unseen[0] != "10.0.0.0" || unseen[1] != "10.0.0.2" || unseen[2] != "10.0.0.3" {
		t.Fatalf("unexpected expected_but_unseen list: %+v", unseen)
	}

	seenOut := result.Lists.SeenButOutOfScope
	if len(seenOut) != 2 || seenOut[0].IPAddress != "10.0.0.4" || seenOut[1].IPAddress != "10.0.0.5" {
		t.Fatalf("unexpected seen_but_out_of_scope list: %+v", seenOut)
	}
	if len(result.Lists.SeenButOutOfScopeAndMarkedInScope) != 1 || result.Lists.SeenButOutOfScopeAndMarkedInScope[0].IPAddress != "10.0.0.5" {
		t.Fatalf("unexpected marked-in-scope list: %+v", result.Lists.SeenButOutOfScopeAndMarkedInScope)
	}
	if len(result.Lists.SeenButOutOfScopeAndMarkedOutOfScope) != 1 || result.Lists.SeenButOutOfScopeAndMarkedOutOfScope[0].IPAddress != "10.0.0.4" {
		t.Fatalf("unexpected marked-out-of-scope list: %+v", result.Lists.SeenButOutOfScopeAndMarkedOutOfScope)
	}
}
