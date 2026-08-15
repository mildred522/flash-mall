-- DTM 1.19.0 MySQL storage schema.
-- Keep this module idempotent: docker startup applies schema.sql on every run.
-- Do not copy DTM's reset script here because it drops active transaction data.
USE dtm;

CREATE TABLE IF NOT EXISTS trans_global (
  `id` bigint(22) NOT NULL AUTO_INCREMENT,
  `gid` varchar(128) NOT NULL COMMENT 'global transaction id',
  `trans_type` varchar(45) NOT NULL COMMENT 'transaction type: saga | xa | tcc | msg',
  `status` varchar(12) NOT NULL COMMENT 'transaction status: prepared | submitted | aborting | succeed | failed',
  `query_prepared` varchar(1024) NOT NULL COMMENT 'url to check for msg|workflow',
  `protocol` varchar(45) NOT NULL COMMENT 'protocol: http | grpc | json-rpc',
  `create_time` datetime DEFAULT NULL,
  `update_time` datetime DEFAULT NULL,
  `finish_time` datetime DEFAULT NULL,
  `rollback_time` datetime DEFAULT NULL,
  `options` varchar(1024) DEFAULT '' COMMENT 'transaction options',
  `custom_data` varchar(1024) DEFAULT '' COMMENT 'custom transaction data',
  `next_cron_interval` int(11) DEFAULT NULL COMMENT 'next cron interval',
  `next_cron_time` datetime DEFAULT NULL COMMENT 'next cron execution time',
  `owner` varchar(128) NOT NULL DEFAULT '' COMMENT 'transaction lock owner',
  `ext_data` text COMMENT 'extra transaction data',
  `result` varchar(1024) DEFAULT '' COMMENT 'transaction result',
  `rollback_reason` varchar(1024) DEFAULT '' COMMENT 'rollback reason',
  PRIMARY KEY (`id`),
  UNIQUE KEY `gid` (`gid`),
  KEY `owner` (`owner`),
  KEY `status_next_cron_time` (`status`, `next_cron_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS trans_branch_op (
  `id` bigint(22) NOT NULL AUTO_INCREMENT,
  `gid` varchar(128) NOT NULL COMMENT 'global transaction id',
  `url` varchar(1024) NOT NULL COMMENT 'branch operation URL',
  `data` text COMMENT 'deprecated request body',
  `bin_data` blob COMMENT 'request body',
  `branch_id` varchar(128) NOT NULL COMMENT 'transaction branch ID',
  `op` varchar(45) NOT NULL COMMENT 'action | compensate | try | confirm | cancel',
  `status` varchar(45) NOT NULL COMMENT 'prepared | succeed | failed',
  `finish_time` datetime DEFAULT NULL,
  `rollback_time` datetime DEFAULT NULL,
  `create_time` datetime DEFAULT NULL,
  `update_time` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `gid_uniq` (`gid`, `branch_id`, `op`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS kv (
  `id` bigint(22) NOT NULL AUTO_INCREMENT,
  `cat` varchar(45) NOT NULL COMMENT 'data category',
  `k` varchar(128) NOT NULL,
  `v` text,
  `version` bigint(22) DEFAULT 1 COMMENT 'value version',
  `create_time` datetime DEFAULT NULL,
  `update_time` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_k` (`cat`, `k`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
