/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { Component, createElement } from 'react';
import hoistStatics from 'hoist-non-react-statics';
import PropTypes from 'prop-types';

export const FluxContext = React.createContext();

export const Provider = FluxContext.Provider;

const reactorShape = PropTypes.shape({
  dispatch: PropTypes.func.isRequired,
  evaluate: PropTypes.func.isRequired,
  evaluateToJS: PropTypes.func.isRequired,
  observe: PropTypes.func.isRequired,
})

function getDisplayName(WrappedComponent) {
  return WrappedComponent.displayName || WrappedComponent.name || 'Component'
}

export function connect(mapFluxToProps, mapStateToProps) {
  mapStateToProps = mapStateToProps ? mapStateToProps : () => ({});
  return function wrapWithConnect(WrappedComponent) {
    class Connect extends Component {

      constructor(props, context) {
        super(props)
        this.reactor = props.reactor || context
        this.unsubscribeFns = []
        this.updatePropMap(props)
      }

      UNSAFE_componentWillMount() {
        this.updateState()
        this.subscribe(this.props)
      }

      componentWillUnmount() {
        this.unsubscribe()
      }

      updatePropMap(props) {
        this.propMap = (mapFluxToProps) ? mapFluxToProps(props) : {}
      }

      updateState() {
        let propMap = this.propMap
        let stateToSet = {}

        for (let key in propMap) {
          const getter = propMap[key]
          stateToSet[key] = this.reactor.evaluate(getter)
        }

        this.setState(stateToSet)
      }

      subscribe() {
        let propMap = this.propMap
        for (let key in propMap) {
          const getter = propMap[key]
          const unsubscribeFn = this.reactor.observe(getter, val => {
            this.setState({
              [key]: val,
            })
          })

          this.unsubscribeFns.push(unsubscribeFn)
        }
      }

      unsubscribe() {
        if (this.unsubscribeFns.length === 0) {
          return
        }

        while (this.unsubscribeFns.length > 0) {
          this.unsubscribeFns.shift()()
        }
      }

      render() {
        const stateProps = mapStateToProps(this.props);
        return createElement(WrappedComponent, {
          reactor: this.reactor,
          ...stateProps,
          ...this.props,
          ...this.state,
        })
      }
    }

    Connect.contextType = FluxContext;
    Connect.displayName = `Connect(${getDisplayName(WrappedComponent)})`
    Connect.WrappedComponent = WrappedComponent
    Connect.propTypes = {
      reactor: reactorShape,
    }

    return hoistStatics(Connect, WrappedComponent)
  }
}


export function useFluxStore(getter) {
  const reactor = React.useContext(FluxContext);
  const [ state, setState ] = React.useState(() => {
    return reactor.evaluate(getter)
  });

  React.useLayoutEffect(() => {
    function subscribe(){
      return reactor.observe(getter, val => {
        setState(val)
      });
    }

    const newState = reactor.evaluate(getter);

    setState(newState);

    const unsubscribe = subscribe();

    return unsubscribe;
  }, []);

  return state;
}