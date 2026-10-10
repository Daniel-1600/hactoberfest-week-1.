import type { Metadata } from 'next';
import './globals.css';

export const metadata: Metadata = {
  title: 'EcoSound — A little closer to nature',
  description: 'Listen to the birds around you, identify their calls with BirdNET, and build your own nature journal.',
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body>{children}</body></html>;
}
