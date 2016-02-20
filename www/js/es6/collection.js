'use strict';System.register(['./model'],function(_export,_context){var Model;return {setters:[function(_model){Model=_model.default;}],execute:function(){class Collection{constructor(models){this.models={};if(models){for(let model of models){this.add(model);}}}add(model){this.models[model.id]=model;model.collection=this;}remove(model){delete this.models[model.id];delete model.collection;}clear(){for(let id of this.models){delete this.models[id].collection;}this.models={};}forEach(fn){for(let id in this.models){fn(this.models[id]);}}}_export('default',Collection);}};});
//# sourceMappingURL=maps/collection.js.map