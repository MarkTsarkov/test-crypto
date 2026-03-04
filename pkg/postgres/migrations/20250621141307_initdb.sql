-- +goose Up
-- +goose StatementBegin
CREATE TABLE banners (
    banner_id SERIAL PRIMARY KEY,
    banner_name TEXT NOT NULL
);

CREATE TABLE click_stats
(
    id        SERIAL    PRIMARY KEY,
    banner_id INT       NOT NULL,
    ts        TIMESTAMP NOT NULL,
    count     INT       NOT NULL,
    FOREIGN KEY (banner_id) REFERENCES banners (banner_id),
    UNIQUE (banner_id, ts)
);


INSERT INTO banners (banner_name)
    SELECT 'Banner ' || generate_series(1, 100) AS banner_name;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE click_stats;
DROP TABLE banners;
-- +goose StatementEnd
