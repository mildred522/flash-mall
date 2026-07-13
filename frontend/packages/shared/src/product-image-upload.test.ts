import { afterEach, describe, expect, it, vi } from 'vitest';
import { setToken } from './auth';
import { uploadProductImage } from './product-image-upload';

describe('uploadProductImage', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
  });

  it('uploads the image as multipart data and unwraps the Hertz response', async () => {
    setToken('merchant-token');
    const file = new File(['png'], 'coat.png', { type: 'image/png' });
    let requestURL = '';
    let requestBody: FormData | undefined;

    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      requestURL = String(input);
      requestBody = init?.body as FormData;
      expect(new Headers(init?.headers).get('Authorization')).toBe('Bearer merchant-token');
      expect(new Headers(init?.headers).has('Content-Type')).toBe(false);
      return new Response(JSON.stringify({
        code: 'OK',
        data: { image_url: '/uploads/products/coat.png' },
      }), { status: 200, headers: { 'Content-Type': 'application/json' } });
    }));

    const imageURL = await uploadProductImage(file, '/api/merchant/products/image');

    expect(imageURL).toBe('/uploads/products/coat.png');
    expect(requestURL).toBe('/api/merchant/products/image');
    expect(requestBody?.get('image')).toBe(file);
  });

  it('rejects files larger than five megabytes before sending a request', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    const file = new File([new Uint8Array(5 * 1024 * 1024 + 1)], 'large.png', { type: 'image/png' });

    await expect(uploadProductImage(file, '/api/admin/products/image'))
      .rejects.toThrow('图片不能超过 5 MB');
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
