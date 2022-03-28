ALTER TABLE flows MODIFY COLUMN TimeReceived CODEC(DoubleDelta, LZ4)
