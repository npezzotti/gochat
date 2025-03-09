CREATE TABLE accounts(
  id SERIAL PRIMARY KEY,
  username VARCHAR(50) UNIQUE NOT NULL,
  email VARCHAR(50) UNIQUE NOT NULL,
  password_hash VARCHAR(100) NOT NULL,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);

CREATE TABLE rooms(
  id SERIAL PRIMARY KEY
  owner_id REFERENCES accounts(id),
	name email VARCHAR(50) UNIQUE NOT NULL,
	description VARCHAR(100) NOT NULL,
	created_at TIMESTAMP
  updated_at TIMESTAMP
);
