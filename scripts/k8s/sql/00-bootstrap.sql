-- 本目录 SQL 模块是数据库初始化源码；修改后运行 node scripts/k8s/build-sql-bundles.mjs。
-- scripts/k8s/schema.sql 仅聚合可重复执行的结构迁移；演示数据单独生成到 demo-seed.sql。
-- 初始化数据库与表结构（K8s MySQL）

SET NAMES utf8mb4;
SET character_set_client = utf8mb4;
SET character_set_connection = utf8mb4;
SET character_set_results = utf8mb4;
SET collation_connection = utf8mb4_general_ci;

CREATE DATABASE IF NOT EXISTS mall_order DEFAULT CHARSET utf8mb4;
CREATE DATABASE IF NOT EXISTS mall_product DEFAULT CHARSET utf8mb4;
CREATE DATABASE IF NOT EXISTS mall_auth DEFAULT CHARSET utf8mb4;
CREATE DATABASE IF NOT EXISTS dtm DEFAULT CHARSET utf8mb4;

ALTER DATABASE mall_order CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
ALTER DATABASE mall_product CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
ALTER DATABASE mall_auth CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
ALTER DATABASE dtm CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
