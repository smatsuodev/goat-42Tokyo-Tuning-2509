-- このファイルに記述されたSQLコマンドが、マイグレーション時に実行されます。

CREATE INDEX idx_shippped_status ON orders (shipped_status);
CREATE INDEX idx_user_name ON users (user_name);

CREATE TABLE cache (
    target VARCHAR(255) PRIMARY KEY,
);