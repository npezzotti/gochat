CREATE TABLE rooms (
    id SERIAL PRIMARY KEY,
    owner_id integer REFERENCES accounts(id),
    name character varying(50) NOT NULL,
    description character varying(100) NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    seq_id integer DEFAULT 0 NOT NULL,
    external_id character varying(50)
);
CREATE INDEX idx_rooms_external_id ON rooms(external_id);