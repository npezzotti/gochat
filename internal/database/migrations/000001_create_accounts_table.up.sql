CREATE TABLE accounts(
    id SERIAL PRIMARY KEY,
    username character varying(50) NOT NULL,
    email character varying(50) NOT NULL,
    password_hash character varying(100) NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX idx_accounts_email ON accounts(email);
