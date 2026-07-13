import { getToken } from './auth';

export const MAX_PRODUCT_IMAGE_BYTES = 5 * 1024 * 1024;

type UploadEnvelope = {
  message?: string;
  data?: { image_url?: string };
  image_url?: string;
};

export async function uploadImageAsset(
  file: File,
  options: { endpoint: string; fields?: Record<string, string> },
): Promise<string> {
  if (!file || file.size === 0) throw new Error('请选择图片文件');
  if (file.size > MAX_PRODUCT_IMAGE_BYTES) throw new Error('图片不能超过 5 MB');
  if (!file.type.startsWith('image/')) throw new Error('只支持图片文件');

  const token = getToken();
  if (!token) throw new Error('登录已失效，请重新登录');

  const body = new FormData();
  body.append('image', file);
  Object.entries(options.fields ?? {}).forEach(([key, value]) => body.append(key, value));
  const response = await fetch(options.endpoint, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body,
  });

  const payload = await response.json().catch(() => ({})) as UploadEnvelope;
  if (!response.ok) throw new Error(payload.message || '图片上传失败');
  const imageURL = payload.data?.image_url || payload.image_url || '';
  if (!imageURL) throw new Error('图片上传响应缺少地址');
  return imageURL;
}

export function uploadProductImage(file: File, endpoint: string): Promise<string> {
  return uploadImageAsset(file, { endpoint });
}
