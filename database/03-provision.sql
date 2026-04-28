ALTER TABLE tenants
ADD COLUMN `status` VARCHAR(15) NOT NULL DEFAULT 'pending',
ADD INDEX idx_status (`status`);