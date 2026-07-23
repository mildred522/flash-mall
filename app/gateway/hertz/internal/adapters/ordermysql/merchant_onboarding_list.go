package ordermysql

import (
	"context"

	"flash-mall/app/gateway/hertz/internal/application/merchantonboarding"
)

func (r *MerchantOnboardingRepository) List(
	ctx context.Context,
	query merchantonboarding.ListQuery,
) (merchantonboarding.ListResult, error) {
	where := "1=1"
	args := make([]any, 0, 1)
	if query.Status >= 0 {
		where += " AND status = ?"
		args = append(args, query.Status)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM merchant_apply WHERE "+where, args...).Scan(&total); err != nil {
		return merchantonboarding.ListResult{}, err
	}
	queryArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, merchant_name, contact_phone, status, merchant_id,
       audit_remark, operator_id,
       COALESCE(DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s'), ''),
       COALESCE(DATE_FORMAT(audit_time, '%Y-%m-%d %H:%i:%s'), '')
FROM merchant_apply
WHERE `+where+`
ORDER BY CASE WHEN status = 0 THEN 0 ELSE 1 END, id DESC
LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return merchantonboarding.ListResult{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]merchantonboarding.Application, 0)
	for rows.Next() {
		var item merchantonboarding.Application
		if err := rows.Scan(&item.ApplyID, &item.UserID, &item.MerchantName, &item.ContactPhone,
			&item.Status, &item.MerchantID, &item.AuditRemark, &item.OperatorID,
			&item.CreateTime, &item.AuditTime); err != nil {
			return merchantonboarding.ListResult{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return merchantonboarding.ListResult{}, err
	}
	return merchantonboarding.ListResult{
		Items: items, Total: total, Page: query.Page, PageSize: query.PageSize,
	}, nil
}
