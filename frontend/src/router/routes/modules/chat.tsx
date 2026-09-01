import { lazy, Suspense } from 'react';

import { CircleLoading } from '@/components/loading';
import Iconify from '@/components/icon/iconify-icon';
import { AppRouteObject } from '#/router';

const ChatPage = lazy(() => import('@/pages/chat'));

const chat: AppRouteObject = {
  path: '/admin/chat',
  element: (
    <Suspense fallback={<CircleLoading />}>
      <ChatPage />
    </Suspense>
  ),
  order: 1,
  meta: {
    label: 'ChatGPT',
    icon: <Iconify icon="simple-icons:openai" className="ant-menu-item-icon" size="24" />,
    key: '/admin/chat',
  },
};

export default chat;
