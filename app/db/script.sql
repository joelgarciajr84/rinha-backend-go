set timezone TO 'UTC';

create type processor_type as ENUM ('primary', 'fallback');

create unlogged table financial_transaction (
	correlation_id UUID primary key,
	amount DECIMAL not null,
	processor_type processor_type not null,
	processed_at timestamp not null default NOW()
);

create index idx_processed_at on financial_transaction using btree (processed_at);
create index idx_processor_type on financial_transaction using btree (processor_type);
create index idx_processed_at_processor on financial_transaction using btree (processed_at, processor_type);
