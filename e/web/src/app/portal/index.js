import Portal from './components/main.jsx';
import AppsAndSites from './components/appsAndSites.jsx';
import {initPortal} from './flux/actions';
import './flux';

const routes = [
  {
    title: 'Ops Center',
    onEnter: initPortal,
    component: Portal,
    indexRoute: {
      component: AppsAndSites
    }
  }
]

export default routes;
