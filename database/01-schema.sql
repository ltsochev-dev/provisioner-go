CREATE TABLE IF NOT EXISTS tenants (
    id CHAR(36) NOT NULL,
    email VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    domain VARCHAR(255) NOT NULL,
    plan VARCHAR(64) NOT NULL,
    subscribed_until DATE NULL,
    subscribed_at DATE NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_tenants_email (email),
    UNIQUE KEY uq_tenants_slug (slug),
    UNIQUE KEY uq_tenants_domain (domain)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS tenant_keys (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT,
    tenant_id CHAR(36) NOT NULL,
    `key` VARCHAR(255) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_tenant_keys_key (`key`),
    KEY idx_tenant_keys_tenant_id (tenant_id),
    CONSTRAINT fk_tenant_keys_tenant_id
        FOREIGN KEY (tenant_id)
        REFERENCES tenants (id)
        ON DELETE CASCADE
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
