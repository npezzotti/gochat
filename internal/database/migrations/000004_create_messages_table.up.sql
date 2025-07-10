CREATE TABLE messages(
  id        SERIAL PRIMARY KEY,
  seq_id integer NOT NULL,
  room_id integer NOT NULL,
  user_id integer NOT NULL,
  content character varying(100),
  created_at timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX messages_room_seq_id ON messages(room_id, seq_id);
