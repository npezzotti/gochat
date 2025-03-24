CREATE TABLE accounts(
  id SERIAL PRIMARY KEY,
  username VARCHAR(50) UNIQUE NOT NULL,
  email VARCHAR(50) UNIQUE NOT NULL,
  password_hash VARCHAR(100) NOT NULL,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);

CREATE TABLE rooms (
    id SERIAL PRIMARY KEY,
    owner_id integer,
    name character varying(50) NOT NULL,
    description character varying(100) NOT NULL,
    seq_id integer DEFAULT 0 NOT NULL
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    FOREIGN KEY (owner_id) REFERENCES accounts(id);
);

CREATE TABLE IF NOT EXISTS subscriptions(
  id         SERIAL PRIMARY KEY,
  account_id INT NOT NULL,
  room_id    INT NOT NULL,
  created_at TIMESTAMP(3) NOT NULL,
  updated_at TIMESTAMP(3) NOT NULL,
  FOREIGN KEY(account_id) REFERENCES accounts(id),
  FOREIGN KEY(room_id) REFERENCES rooms(id)
);
CREATE UNIQUE INDEX subscriptions_room_user_id ON subscriptions(account_id, room_id);

CREATE TABLE messages(
  id        SERIAL PRIMARY KEY,
  seq_id     INT NOT NULL,
  room_id     INT NOT NULL,
  user_id integer NOT NULL,
  content   VARCHAR(100),
  created_at TIMESTAMP(3) NOT NULL,
  updated_at TIMESTAMP(3) NOT NULL,
  FOREIGN KEY(room_id) REFERENCES rooms(id)
  FOREIGN KEY(from) REFERENCES accounts(id)
);
CREATE UNIQUE INDEX messages_room_seq_id ON messages(room_id, seq_id);
