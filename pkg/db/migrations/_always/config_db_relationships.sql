--- Add config_item foreign key to config_component_relationships table
DO $$
BEGIN
IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'config_items')
    THEN
    ALTER TABLE config_component_relationships ADD FOREIGN KEY (config_id) REFERENCES config_items(id);
END IF;
END $$;
