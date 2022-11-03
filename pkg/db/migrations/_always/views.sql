-- CREATE OR REPLACE VIEW  checks_by_component AS

--       SELECT check_component_relationships.component_id, json_agg(checks) from checks
--             LEFT JOIN check_component_relationships ON checks.id = check_component_relationships.check_id
--             WHERE    check_component_relationships.deleted_at is null
--             GROUP BY check_component_relationships.component_id;


-- CREATE OR REPLACE VIEW components_flat AS
-- 	SELECT components.id, components.type, components.name, jsonb_set_lax(to_jsonb(components),'{checks}',
-- 			(SELECT json_agg(checks) from checks LEFT JOIN check_component_relationships ON checks.id = check_component_relationships.check_id WHERE check_component_relationships.component_id = components.id AND check_component_relationships.deleted_at is null   GROUP BY check_component_relationships.component_id) :: jsonb
-- 			 ) :: jsonb as components from components where components.deleted_at is null;

-- select * from components_flat


CREATE OR REPLACE function lookup_component_by_property(text, text)
returns setof components
as
$$
begin
  return query
    select * from components where deleted_at is null AND properties != 'null' and name in (select name  from components,jsonb_array_elements(properties) property where properties != 'null' and  property is not null and  property->>'name' = $1 and property->>'text' = $2);
end;
$$
language plpgsql;



CREATE OR REPLACE VIEW component_names AS
      SELECT id, external_id, type, name, created_at, updated_at, icon, parent_id FROM components where deleted_at is null AND hidden != true ;

CREATE OR REPLACE VIEW component_labels AS
      SELECT DISTINCT labels FROM components where deleted_at is null AND hidden != true ;


CREATE OR REPLACE VIEW check_names AS
      SELECT id, canary_id, type, name, status FROM checks where deleted_at is null AND silenced_at is null;

