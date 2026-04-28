INSERT INTO tenants (
    id,
    email,
    name,
    slug,
    domain,
    plan,
    subscribed_until,
    subscribed_at
) VALUES
    (
        '11111111-1111-4111-8111-111111111111',
        'admin@acme.example',
        'Acme Ltd',
        'acme',
        'acme.example',
        'starter',
        '2027-04-28',
        '2026-04-28'
    ),
    (
        '22222222-2222-4222-8222-222222222222',
        'admin@globex.example',
        'Globex Corporation',
        'globex',
        'globex.example',
        'business',
        '2027-04-28',
        '2026-04-28'
    );

INSERT INTO tenant_keys (
    tenant_id,
    `key`
) VALUES
    (
        '11111111-1111-4111-8111-111111111111',
        'tenant_acme_dev_key_111111111111'
    ),
    (
        '11111111-1111-4111-8111-111111111111',
        'tenant_acme_secondary_key_111111111111'
    ),
    (
        '22222222-2222-4222-8222-222222222222',
        'tenant_globex_dev_key_222222222222'
    );
