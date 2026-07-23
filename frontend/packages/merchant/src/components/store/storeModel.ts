import type { MerchantStoreProfile } from '@flash-mall/shared';

export type StoreFormValues = Pick<MerchantStoreProfile, 'logo_url' | 'banner_url' | 'description'>;
export type StoreAssetType = 'logo' | 'banner';
