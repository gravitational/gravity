import api from 'oss-app/services/api';
import cfg from 'e-app/config';

export function createLicense({ amount, expiration, isStrict } ) {
  const data = {
    max_nodes: amount,
    expiration: expiration,
    stop_app: isStrict
  }

  return api.post(cfg.api.licenseGeneratorPath, data)
    .then(res => {
      return res.license;
    })
}