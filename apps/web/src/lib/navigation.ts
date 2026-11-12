const pages: Record<string, string> = {
  Overview: '/',
  Journeys: '/journeys',
  Templates: '/templates',
  Analytics: '/analytics',
  Customers: '/customers',
  Events: '/events',
  'Test Lab': '/test-lab',
  'Test Inbox': '/test-inbox',
  Settings: '/settings',
};
const tabs: Record<string, string> = {
  Builder: 'builder',
  'Preview & Test': 'preview-test',
  'Live Monitor': 'live-monitor',
  Analytics: 'analytics',
  Versions: 'versions',
};
const statuses = ['Draft', 'Active', 'Paused', 'Sunset', 'Completed', 'Archived'];
export type InspectionKind = 'event' | 'execution' | 'message';
export type WorkspaceRoute = {
  create?: boolean;
  customerID?: string;
  executionID?: string;
  deliveryID?: string;
  inspect?: InspectionKind;
  record?: string;
  page: string;
  journeyID?: string;
  tab?: string;
  preview?: string;
  status?: string;
};
export function pageRoute(page: string): WorkspaceRoute {
  return { page: Object.hasOwn(pages, page) ? page : 'Overview' };
}
export function readRoute(url: URL): WorkspaceRoute {
  const inspect = url.searchParams.get('inspect');
  const record = url.searchParams.get('record') || '';
  const inspection =
    ['event', 'execution', 'message'].includes(inspect || '') &&
    record.length > 0 &&
    record.length <= 128
      ? { inspect: inspect as InspectionKind, record }
      : {};
  const path = url.pathname.replace(/\/+$/, '') || '/';
  if (path === '/journeys/new') return { page: 'Journeys', create: true };
  const journey = path.match(/^\/journeys\/([0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12})$/i);
  if (journey) {
    return {
      ...inspection,
      executionID: url.searchParams.get('execution') || undefined,
      deliveryID: url.searchParams.get('delivery') || undefined,
      page: 'Journeys',
      journeyID: journey[1],
      tab: Object.keys(tabs).find((key) => tabs[key] === url.searchParams.get('tab')) || 'Builder',
      preview: url.searchParams.get('view') === 'test-lab' ? 'Test Lab' : 'Messages',
    };
  }
  const page = Object.keys(pages).find((key) => pages[key] === path) || 'Overview';
  const status = url.searchParams.get('status') || '';
  return {
    page,
    ...inspection,
    ...(page === 'Customers' ? { customerID: url.searchParams.get('customer') || undefined } : {}),
    ...(page === 'Journeys' && statuses.includes(status) ? { status } : {}),
  };
}
export function routeURL(route: WorkspaceRoute): string {
  let path = pages[route.page] || '/';
  const params = new URLSearchParams();
  if (route.page === 'Journeys' && route.create) return '/journeys/new';
  if (route.page === 'Journeys' && route.journeyID) {
    path += '/' + encodeURIComponent(route.journeyID);
    if (route.tab && route.tab !== 'Builder' && tabs[route.tab]) params.set('tab', tabs[route.tab]);
    if (route.tab === 'Preview & Test' && route.preview === 'Test Lab')
      params.set('view', 'test-lab');
  } else if (route.page === 'Journeys' && route.status && statuses.includes(route.status))
    params.set('status', route.status);
  if (route.page === 'Customers' && route.customerID) params.set('customer', route.customerID);
  if (route.journeyID && route.executionID) params.set('execution', route.executionID);
  if (route.journeyID && route.deliveryID) params.set('delivery', route.deliveryID);
  if (route.inspect && route.record) {
    params.set('inspect', route.inspect);
    params.set('record', route.record);
  }
  return path + (params.size ? '?' + params : '');
}
