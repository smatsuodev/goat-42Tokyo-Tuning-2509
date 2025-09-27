-- このファイルに記述されたSQLコマンドが、マイグレーション時に実行されます。

CREATE INDEX idx_shippped_status ON orders (shipped_status);
CREATE INDEX idx_user_name ON users (user_name);
CREATE INDEX idx_name_id ON products (name, product_id);
CREATE INDEX idx_name_idd ON products (name DESC, product_id);
CREATE INDEX idx_value_id ON products (value, product_id);
CREATE INDEX idx_value_idd ON products (value DESC, product_id);
CREATE INDEX idx_image_id ON products (image, product_id);
CREATE INDEX idx_image_idd ON products (image DESC, product_id);
CREATE INDEX idx_weight_id ON products (weight, product_id);
CREATE INDEX idx_weight_idd ON products (weight DESC, product_id);

CREATE TABLE cache (
    target VARCHAR(255) PRIMARY KEY
);

SELECT SLEEP(60);

DROP TABLE cache;