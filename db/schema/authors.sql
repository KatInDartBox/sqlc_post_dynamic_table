CREATE TABLE cashflow (
  id   BIGSERIAL PRIMARY KEY,
  name text      NOT NULL,
  credit_ref BIGSERIAL,
  total  numeric(15,2) not null default 0
);

CREATE TABLE to_cashflow (
  id   BIGSERIAL PRIMARY KEY,
  name text      NOT NULL,
  credit_ref BIGSERIAL,
  total  numeric(15,2) not null default 0
);


CREATE TABLE cash_credit (
  id   BIGSERIAL PRIMARY KEY,
  name text      NOT NULL,
  credit_ref BIGSERIAL,
  total  numeric(15,2) not null default 0
);

CREATE TABLE cash_inex (
  id   BIGSERIAL PRIMARY KEY,
  name text      NOT NULL,
  credit_ref BIGSERIAL,
  total  numeric(15,2) not null default 0
);



create table ref_cash(
  id   BIGSERIAL PRIMARY KEY,
  name text      NOT NULL,
  credit_ref BIGSERIAL,
  total  numeric(15,2) not null default 0
);

CREATE TABLE income (
  id   BIGSERIAL PRIMARY KEY,
  name text      NOT NULL,
  credit_ref BIGSERIAL,
  total  numeric(15,2) not null default 0
);

