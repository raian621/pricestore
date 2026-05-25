CREATE TABLE candlesticks (
  asset_id BIGINT NOT NULL REFERENCES asset_metadata ON DELETE CASCADE,
  start_usec BIGINT NOT NULL,
  end_usec BIGINT NOT NULL,
  open_price DOUBLE PRECISION NOT NULL,
  high_price DOUBLE PRECISION NOT NULL,
  low_price DOUBLE PRECISION NOT NULL,
  close_price DOUBLE PRECISION NOT NULL,
  volume DOUBLE PRECISION NOT NULL
) WITH (
  tsdb.hypertable,
  tsdb.segmentby = 'asset_id',
  tsdb.orderby = 'start_usec DESC',
  tsdb.partition_column = 'start_usec',
  -- 7 days:
  tsdb.chunk_interval = 604800
);
