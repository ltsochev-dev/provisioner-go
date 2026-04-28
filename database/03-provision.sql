ALTER TABLE tenants
ADD COLUMN `status` VARCHAR(15) NOT NULL DEFAULT 'pending',
ADD COLUMN `locked_at` DATETIME(6) NULL
ADD INDEX idx_status (`status`);