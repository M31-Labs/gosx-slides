const test=require('node:test'),assert=require('node:assert/strict');
const {sampleCommands}=require('../assets/graphics-motion.js');
const create=(id,x,color='#00ff00')=>({kind:0,objectId:id,data:{geometry:'lines',props:{id,points:[{x:0,y:0,z:0},{x,y:2,z:1}],lineSegments:[[0,1]],color}}});
test('absolute poses sample attached geometry, camera and shader uniforms independent of seek order',()=>{
 const from=[{kind:1,objectId:'edge'},create('edge',2),{kind:5,data:{z:10,fov:50}},{kind:14,data:{uniforms:{gain:0}}}];
 const to=[{kind:1,objectId:'edge'},create('edge',6,'#ff0000'),{kind:5,data:{z:6,fov:40}},{kind:14,data:{uniforms:{gain:1}}}];
 const mid=sampleCommands(from,to,.5);assert.equal(mid.filter(c=>c.objectId==='edge').length,1);assert.equal(mid[0].data.props.points[1].x,4);assert.deepEqual(mid[0].data.props.lineSegments,[[0,1]]);assert.equal(mid.find(c=>c.kind===5).data.z,8);assert.equal(mid.find(c=>c.kind===14).data.uniforms.gain,.5);
 sampleCommands(from,to,1);sampleCommands(from,to,.1);assert.deepEqual(sampleCommands(from,to,.5),mid);assert.deepEqual(sampleCommands(from,to,0)[0],create('edge',2));assert.deepEqual(sampleCommands(from,to,1)[0],create('edge',6,'#ff0000'));
 assert.equal(from[1].data.props.points[1].x,2,'source mutated');
});
test('hidden actor state settles exactly at the endpoint',()=>{const from=[create('actor',2)],to=[{kind:1,objectId:'actor'}];assert.equal(sampleCommands(from,to,.5)[0].kind,0);assert.deepEqual(sampleCommands(from,to,1),to);});
test('omitted IR coordinates interpolate from zero and restore explicit zero on rewind',()=>{
 const label=z=>({kind:0,objectId:'caption',data:{kind:'label',props:{id:'caption',text:'route',x:1,...(z===undefined?{}:{z})}}});
 const from=[label(),{kind:0,objectId:'edge',data:{geometry:'lines',props:{points:[{x:1,y:2}]}}}],to=[label(2),{kind:0,objectId:'edge',data:{geometry:'lines',props:{points:[{x:1,y:2,z:4}]}}}];
 const mid=sampleCommands(from,to,.5);assert.equal(mid[0].data.props.z,1);assert.equal(mid[1].data.props.points[0].z,2);
 assert.equal(sampleCommands(from,to,0)[0].data.props.z,0);assert.equal(sampleCommands(from,to,0)[1].data.props.points[0].z,0);assert.equal(from[0].data.props.z,undefined);
});
