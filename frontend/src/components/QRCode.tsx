import { useEffect, useRef } from 'react';
import QRCodeLib from 'qrcode';

interface QRCodeProps {
  value: string;
  size?: number;
  className?: string;
}

/**
 * Locally-generated QR code (no third-party service).
 *
 * Replaces the previous `https://api.qrserver.com/v1/create-qr-code/?data=...`
 * approach which leaked subscription credentials (sub_token, SS passwords,
 * VMess UUIDs) to a third party via GET query strings.
 *
 * Uses the `qrcode` npm package which renders directly to a <canvas> in the
 * browser — the encoded value never leaves the user's device.
 */
export default function QRCode({ value, size = 160, className }: QRCodeProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    if (!canvasRef.current || !value) return;
    QRCodeLib.toCanvas(
      canvasRef.current,
      value,
      { width: size, margin: 1, errorCorrectionLevel: 'M' },
      (err) => {
        if (err) {
          /* silent fail — leave canvas blank rather than crash the UI */
        }
      },
    );
  }, [value, size]);

  return (
    <canvas
      ref={canvasRef}
      width={size}
      height={size}
      className={className}
      role="img"
      aria-label="QR code"
    />
  );
}
