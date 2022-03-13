CREATE TABLE IF NOT EXISTS user (
  user_id integer primary key autoincrement,
  username string not null,
  email string not null,
  pw_hash string not null
);

CREATE TABLE IF NOT EXISTS follower (
  who_id integer,
  whom_id integer
);

CREATE TABLE IF NOT EXISTS message (
  message_id integer primary key autoincrement,
  author_id integer not null,
  text string not null,
  pub_date integer,
  flagged integer
);
