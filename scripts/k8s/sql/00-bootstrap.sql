-- 本目录 SQL 模块是数据库初始化源码；修改后运行 node scripts/k8s/build-init-db.mjs。
-- scripts/k8s/init-db.sql 是供 Docker、K8s 和冒烟脚本使用的兼容聚合文件。
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
