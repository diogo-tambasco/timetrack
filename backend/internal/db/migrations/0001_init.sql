-- 0001_init: extensões base. Schema de domínio começa na fase 01.
-- gen_random_uuid() pras PKs uuid das fases 01+; explicitamente em public
-- pra resolução funcionar em qualquer search_path (schemas de teste incluídos)
CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;
