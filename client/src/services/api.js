import { Subject } from 'rxjs';
import {  map, take, filter } from 'rxjs/operators';
import uuid from 'uuid';

class Api {

    constructor() {
        this.broker = new Subject();
        window.response = this.response.bind(this);
        // this.request('START', {});
    }

    request(type= '', content= {}) {
        const reqId = uuid.v4();
        window.Request(type, reqId, JSON.stringify(content));
        return reqId;
    }

    response(type, resId, status, response, err) {
        const res = {
            resId, 
            type, 
            status, 
            err,
            response: JSON.parse(response),
        };
        this.broker.next(res);
    }

    login(details) {
        const reqId = this.request('LOGIN', details);
        return this.broker.asObservable().pipe(
            filter(e => e.resId === reqId),
            filter(e => e.type === 'LOGIN_RESPONSE'),
            map(e => {
                if (e.status === 'SUCCESS') {
                    return e.response;
                } else {
                    throw new Error(e.error);
                }
            }),
            take(1),
        );
    }

    startTap(details) {
        const reqId = this.request('TAP', details);
        return this.broker.asObservable().pipe(
            filter(e => e.resId === reqId),
            filter(e => e.type === 'TAP'),
            map(e => {
                console.log(e);
                return e.response;
            }),
        );
    }
}

const api = new Api();
export default api;