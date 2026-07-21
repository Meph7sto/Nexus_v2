package service

import "testing"

func TestNormalizeAdminPermissions(t *testing.T) {
	input := []AdminPermission{
		{
			Resource: AdminResourceUsers,
			Actions:  []AdminPermissionAction{AdminActionUpdate, AdminActionView},
		},
		{
			Resource: AdminResourceAccounts,
			Actions:  []AdminPermissionAction{AdminActionExecute, AdminActionView},
		},
	}

	got, err := NormalizeAdminPermissions(input)
	if err != nil {
		t.Fatalf("NormalizeAdminPermissions() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(NormalizeAdminPermissions()) = %d, want 2", len(got))
	}
	if got[0].Resource != AdminResourceAccounts || got[1].Resource != AdminResourceUsers {
		t.Fatalf("resources = %#v, want deterministic resource order", got)
	}
	if got[1].Actions[0] != AdminActionView || got[1].Actions[1] != AdminActionUpdate {
		t.Fatalf("actions = %#v, want view before update", got[1].Actions)
	}

	// Normalization must not mutate the caller's order or slices.
	if input[0].Actions[0] != AdminActionUpdate {
		t.Fatalf("NormalizeAdminPermissions mutated input: %#v", input[0].Actions)
	}
}

func TestNormalizeAdminPermissionsRejectsInvalidGrants(t *testing.T) {
	tests := []struct {
		name  string
		input []AdminPermission
	}{
		{
			name: "unknown resource",
			input: []AdminPermission{{
				Resource: "unknown",
				Actions:  []AdminPermissionAction{AdminActionView},
			}},
		},
		{
			name: "unknown action",
			input: []AdminPermission{{
				Resource: AdminResourceUsers,
				Actions:  []AdminPermissionAction{AdminActionView, "promote"},
			}},
		},
		{
			name: "missing view",
			input: []AdminPermission{{
				Resource: AdminResourceUsers,
				Actions:  []AdminPermissionAction{AdminActionUpdate},
			}},
		},
		{
			name: "duplicate action",
			input: []AdminPermission{{
				Resource: AdminResourceUsers,
				Actions:  []AdminPermissionAction{AdminActionView, AdminActionView},
			}},
		},
		{
			name: "duplicate resource",
			input: []AdminPermission{
				{Resource: AdminResourceUsers, Actions: []AdminPermissionAction{AdminActionView}},
				{Resource: AdminResourceUsers, Actions: []AdminPermissionAction{AdminActionView}},
			},
		},
		{
			name: "super admin only resource",
			input: []AdminPermission{{
				Resource: AdminResourceSettings,
				Actions:  []AdminPermissionAction{AdminActionView},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NormalizeAdminPermissions(tt.input); err == nil {
				t.Fatal("NormalizeAdminPermissions() error = nil, want validation error")
			}
		})
	}
}

func TestAdminPermissionDefinitionsAreIndependentCopies(t *testing.T) {
	definitions := AdminPermissionRegistry()
	if len(definitions) == 0 {
		t.Fatal("AdminPermissionRegistry() returned no definitions")
	}

	definitions[0].Label = "mutated"
	definitions[0].Actions[0] = "mutated"

	again := AdminPermissionRegistry()
	if again[0].Label == "mutated" || again[0].Actions[0] == "mutated" {
		t.Fatal("AdminPermissionRegistry() returned mutable shared state")
	}
}
