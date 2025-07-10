CREATE TABLE subscriptions(
  id         SERIAL PRIMARY KEY,
  created_at timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP,
  account_id integer NOT NULL,
  room_id integer NOT NULL,
  last_read_seq_id integer DEFAULT 0 NOT NULL,
  FOREIGN KEY(account_id) REFERENCES accounts(id),
  FOREIGN KEY(room_id) REFERENCES rooms(id)
);
CREATE UNIQUE INDEX subscriptions_room_user_id ON subscriptions(account_id, room_id);
CREATE INDEX idx_subscriptions_room_id ON subscriptions(room_id);
