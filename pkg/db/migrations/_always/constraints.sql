DO $$
BEGIN
IF EXISTS
  (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'person') THEN
  IF  NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'canaries_created_by_fkey') THEN
    ALTER TABLE canaries ADD  FOREIGN KEY (created_by) REFERENCES person(id);
  END IF;
  IF  NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'templates_created_by_fkey') THEN
    ALTER TABLE templates ADD  FOREIGN KEY (created_by) REFERENCES person(id);
  END IF;
    IF  NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'components_created_by_fkey') THEN
    ALTER TABLE components ADD  FOREIGN KEY (created_by) REFERENCES person(id);
  END IF;
END IF;
END $$;
