--- Add config_item foreign key to config_component_relationships table
DO $$
BEGIN
IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'config_items') AND NOT EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name ILIKE 'config_component_relationships_config_id_fkey%' AND table_name='config_component_relationships')
    THEN
    ALTER TABLE config_component_relationships ADD FOREIGN KEY (config_id) REFERENCES config_items(id);
END IF;
END $$;
