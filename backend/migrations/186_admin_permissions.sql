-- Three-role administrator RBAC.
-- Existing administrators retain their full capability by becoming super_admin.

DO $$
DECLARE
  invalid_roles TEXT;
BEGIN
  SELECT string_agg(DISTINCT COALESCE(role, '<null>'), ', ' ORDER BY COALESCE(role, '<null>'))
  INTO invalid_roles
  FROM users
  WHERE role IS NULL OR role NOT IN ('user', 'admin', 'super_admin');

  IF invalid_roles IS NOT NULL THEN
    RAISE EXCEPTION 'unexpected users.role values block admin RBAC migration: %', invalid_roles;
  END IF;

  UPDATE users
  SET role = 'super_admin'
  WHERE role = 'admin';
END $$;

CREATE TABLE IF NOT EXISTS admin_permissions (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  resource VARCHAR(64) NOT NULL,
  actions JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_admin_permissions_actions_array CHECK (jsonb_typeof(actions) = 'array'),
  CONSTRAINT uq_admin_permissions_user_resource UNIQUE (user_id, resource)
);

-- Nexus already has an earlier admin_permissions table under a different
-- migration filename. CREATE TABLE IF NOT EXISTS leaves that table intact, so
-- explicitly reconcile its constraint names and add the JSON shape check that
-- the older schema did not enforce.
DO $$
DECLARE
  legacy_unique_constraint TEXT;
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conrelid = 'admin_permissions'::regclass
      AND conname = 'uq_admin_permissions_user_resource'
  ) THEN
    SELECT conname
    INTO legacy_unique_constraint
    FROM pg_constraint
    WHERE conrelid = 'admin_permissions'::regclass
      AND contype = 'u'
      AND conkey = ARRAY[
        (SELECT attnum FROM pg_attribute WHERE attrelid = 'admin_permissions'::regclass AND attname = 'user_id'),
        (SELECT attnum FROM pg_attribute WHERE attrelid = 'admin_permissions'::regclass AND attname = 'resource')
      ]
    LIMIT 1;

    IF legacy_unique_constraint IS NOT NULL THEN
      EXECUTE format(
        'ALTER TABLE admin_permissions RENAME CONSTRAINT %I TO uq_admin_permissions_user_resource',
        legacy_unique_constraint
      );
    ELSE
      ALTER TABLE admin_permissions
        ADD CONSTRAINT uq_admin_permissions_user_resource UNIQUE (user_id, resource);
    END IF;
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conrelid = 'admin_permissions'::regclass
      AND conname = 'chk_admin_permissions_actions_array'
  ) THEN
    ALTER TABLE admin_permissions
      ADD CONSTRAINT chk_admin_permissions_actions_array
      CHECK (jsonb_typeof(actions) = 'array');
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_admin_permissions_user_id ON admin_permissions(user_id);
CREATE INDEX IF NOT EXISTS idx_admin_permissions_resource ON admin_permissions(resource);
