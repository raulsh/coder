-- Just like the templates table, this is just a jsonb column. It is unfortunate
-- we cannot apply any types to the json schema :(
ALTER TABLE workspaces ADD COLUMN user_acl jsonb NOT NULL default '{}';
ALTER TABLE workspaces ADD COLUMN group_acl jsonb NOT NULL default '{}';
