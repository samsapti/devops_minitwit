LOAD database
    FROM sqlite:///tmp/minitwit.db
    INTO postgresql://minitwit_user:passwd@db/minitwit_db
    CAST type string to varchar drop typemod    
    AFTER LOAD DO
        $$ ALTER TABLE "user" RENAME TO users; $$,
        $$ ALTER TABLE follower RENAME TO followers; $$,
        $$ ALTER TABLE message RENAME TO messages; $$,
        $$ ALTER TABLE users RENAME COLUMN user_id TO id; $$,
        $$ ALTER TABLE messages RENAME COLUMN message_id TO id; $$,
        $$ ALTER TABLE messages RENAME COLUMN pub_date TO date; $$,
        $$ ALTER TABLE followers RENAME COLUMN who_id TO follower_id; $$,
        $$ ALTER TABLE followers RENAME COLUMN whom_id TO follows_id; $$

WITH include drop, create tables, create indexes, reset sequences, no truncate, foreign keys
SET work_mem to '16MB', maintenance_work_mem to '512 MB';