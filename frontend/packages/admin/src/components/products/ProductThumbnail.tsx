import { useState } from 'react';

type Props = {
  src?: string;
  alt: string;
  width?: number;
  height?: number;
};

export default function ProductThumbnail({ src, alt, width = 48, height = 48 }: Props) {
  const [failed, setFailed] = useState(false);
  if (!src || failed) {
    return (
      <span
        aria-label={alt}
        style={{ color: '#999', display: 'inline-flex', width, height, alignItems: 'center', justifyContent: 'center' }}
      >
        图片不可用
      </span>
    );
  }
  return (
    <img
      src={src}
      alt={alt}
      width={width}
      height={height}
      loading="lazy"
      onError={() => setFailed(true)}
      style={{ borderRadius: 6, objectFit: 'cover' }}
    />
  );
}
