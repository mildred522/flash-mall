CREATE TABLE `orders` (
  `id` varchar(64) NOT NULL COMMENT '订单id',
  `request_id` varchar(64) DEFAULT NULL COMMENT '幂等请求id',
  `user_id` bigint(20) NOT NULL DEFAULT '0' COMMENT '用户id',
  `product_id` bigint(20) NOT NULL DEFAULT '0' COMMENT '商品id',
  `amount` int(11) NOT NULL DEFAULT '0' COMMENT '数量',
  `status` tinyint(3) NOT NULL DEFAULT '0' COMMENT '订单状态 0-待支付 1-已支付 2-已关闭',
  `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_request_id` (`request_id`),
  KEY `ix_user_id` (`user_id`),
  KEY `ix_create_time` (`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单表';
