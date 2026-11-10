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
export type WorkspaceRoute = {
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
  const path = url.pathname.replace(/\/+$/, '') || '/';
  const journey = path.match(/^\/journeys\/([0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12})$/i);
  if (journey) {
    return {
      page: 'Journeys',
      journeyID: journey[1],
      tab: Object.keys(tabs).find((key) => tabs[key] === url.searchParams.get('tab')) || 'Builder',
      preview: url.searchParams.get('view') === 'test-lab' ? 'Test Lab' : 'Messages',
    };
  }
  const page = Object.keys(pages).find((key) => pages[key] === path) || 'Overview';
  const status = url.searchParams.get('status') || '';
  return { page, ...(page === 'Journeys' && statuses.includes(status) ? { status } : {}) };
}
export function routeURL(route: WorkspaceRoute): string {
  let path = pages[route.page] || '/';
  const params = new URLSearchParams();
  if (route.page === 'Journeys' && route.journeyID) {
    path += '/' + encodeURIComponent(route.journeyID);
    if (route.tab && route.tab !== 'Builder' && tabs[route.tab]) params.set('tab', tabs[route.tab]);
    if (route.tab === 'Preview & Test' && route.preview === 'Test Lab')
      params.set('view', 'test-lab');
  } else if (route.page === 'Journeys' && route.status && statuses.includes(route.status))
    params.set('status', route.status);
  return path + (params.size ? '?' + params : '');
}
