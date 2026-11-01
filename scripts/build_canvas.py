#!/usr/bin/env python3
"""Regenerate the bilingual native Canvas panel in all dashboard variants."""
import copy
import json
from pathlib import Path
from urllib.parse import quote

ROOT = Path(__file__).resolve().parents[1]
PANEL_ID = 500


def canvas(english=False, dynamic=True):
    def t(sv, en): return en if english else sv
    els, targets, overrides = [], [], []
    uid = 'ctc-myuplink-overview' + ('-en' if english else '')
    tabs = {'pump': t('Värmepump', 'Heat pump'), 'tank': t('Varmvatten', 'Hot water'), 'house': t('Husvärme', 'Space heating')}
    def link(group):
        suffix = 'dtab=' + quote(tabs[group]) if dynamic else 'viewPanel=' + {'pump':'17','tank':'13','house':'12'}[group]
        return {'title': t('Visa detaljer', 'View details'), 'url': '/d/'+uid+'?${__all_variables}&from=${__from}&to=${__to}&'+suffix, 'targetBlank':False, 'oneClick':True}
    def elem(name, kind, x, y, w, h, text='', field=None, size=16, color='#F4F5F8', bg='transparent', stroke=None, radius=0):
        config={'align':'center','valign':'middle','size':size,'color':{'fixed':color},'text':{'mode':'field' if kind=='metric-value' else 'fixed','fixed':text}}
        if field: config['text']['field']=field
        e={'name':name,'type':kind,'config':config,'placement':{'left':x,'top':y,'width':w,'height':h},'constraint':{'horizontal':'left','vertical':'top'},'background':{'color':{'fixed':bg}},'border':{'width':1 if stroke else 0,'radius':radius,'color':{'fixed':stroke or 'transparent'}}}
        els.append(e)
        return e
    def text(name, value, x,y,w=180,h=24,size=15,field=None,color='#A8B3C7'):
        return elem(name,'text',x,y,w,h,value,field,size,color)
    def metric(alias, param, unit, group, x,y,w=150,h=40,size=28,expr=None):
        expression=expr or 'myuplink_system_device_point{job="myuplink",instance=~"$instance",systemID=~"$system",deviceID=~"$device",parameterID="'+param+'"}'
        targets.append({'refId':chr(65+len(targets)),'expr':expression,'legendFormat':alias,'instant':True,'range':False,'editorMode':'code','datasource':{'type':'prometheus','uid':'${datasource}'}})
        overrides.append({'matcher':{'id':'byName','options':alias},'properties':[{'id':'unit','value':unit},{'id':'links','value':[link(group)]}]})
        return elem(alias,'metric-value',x,y,w,h,field=alias,size=size)
    # Stable native shapes: no remote images, external scripts or API actions.
    for x,name,label in [(20,'pump',t('UTOMHUSDEL','OUTDOOR UNIT')),(455,'tank','ECOZENITH i360'),(870,'house',t('HUSVÄRME','SPACE HEATING'))]:
        text(name+'-heading',label,x,12,270,30,20,color='#F4F5F8')
    text('outside-label',t('Utomhus','Outdoor'),35,59,105)
    metric('Outdoor','62000','celsius','pump',145,51,125,40,26)
    # Paired schematic pipes, warm supply and cool return (not measured flow).
    for prefix,left,width in [('pump-pipe',275,220),('house-pipe',655,245)]:
        elem(prefix+'-supply','rectangle',left,166,width,4,bg='#D9A65C')
        elem(prefix+'-return','rectangle',left,244,width,4,bg='#6C9FDE')
    elem('outdoor-unit','rectangle',35,116,240,160,bg='#202B3B',stroke='#5B6F8C',radius=16)
    elem('fan-ring','ellipse',54,135,120,120,bg='#162130',stroke='#5B6F8C')
    elem('fan-inner','ellipse',66,147,96,96,stroke='#5B6F8C')
    for rotation in [0,45,90,135]:
        blade=elem('fan-blade-'+str(rotation),'rectangle',76,190,76,8,bg='#7891B2',radius=4)
        blade['placement']['rotation']=rotation
    elem('fan-hub','ellipse',101,182,26,26,bg='#AFBED1')
    for i in range(6): elem('grille-'+str(i),'rectangle',192,143+i*19,60,4,bg='#5B6F8C')
    for x in (65,220):elem('pump-foot-'+str(x),'rectangle',x,276,32,8,bg='#5B6F8C')
    text('pump-out-label',t('VP ut','Pump outlet'),298,111,175)
    metric('Pump outlet','62037','celsius','pump',298,135,175,30,25)
    text('pump-in-label',t('VP in','Pump inlet'),298,190,175)
    metric('Pump inlet','62027','celsius','pump',298,213,175,30,25)
    # Tank: capacity is a reported percentage, not a physical water level.
    elem('tank-body','rectangle',495,86,160,230,bg='#243245',stroke='#6F839F',radius=24)
    elem('tank-band','rectangle',502,100,146,6,bg='#D9A65C',radius=3)
    text('tank-temperature-label',t('Tanktemperatur','Tank temperature'),501,125,148)
    metric('Tank temperature','62276','celsius','tank',499,150,152,48,32)
    text('capacity-label',t('VV-kapacitet','Hot water capacity'),501,218,148)
    metric('Hot water capacity','62279','percent','tank',501,244,148,40,27)
    # House silhouette and target temperature.
    elem('roof','triangle',888,51,242,100,bg='#28394E',stroke='#6F839F')
    elem('house-body','rectangle',905,151,210,162,bg='#202B3B',stroke='#6F839F',radius=3)
    text('indoor-label',t('Innetemperatur','Indoor temperature'),920,165,180)
    metric('Indoor temperature','62203','celsius','house',920,190,180,42,31)
    text('room-target-label',t('Önskad rumstemperatur','Room setpoint'),915,241,190,24,13)
    metric('Room setpoint','61509','celsius','house',920,266,180,30,23)
    text('target-label',t('Framledning börvärde','Flow setpoint'),696,267,180,24,13)
    metric('Heating setpoint','62007','celsius','house',696,291,180,30,22)
    text('flow-label',t('Framledning','Flow temperature'),696,111,180)
    metric('Heating flow','62011','celsius','house',696,135,180,30,25)
    text('return-label',t('Retur','Return'),696,190,180)
    metric('Heating return','62015','celsius','house',696,213,180,30,25)
    # Bottom readings, grouped with their corresponding component.
    text('compressor-label',t('Kompressor','Compressor'),35,323,120)
    metric('Compressor speed','62193','suffix:rps','pump',35,349,120,36,25)
    text('charge-label',t('Laddpump','Charge pump'),165,323,120)
    metric('Charge pump','62107','percent','pump',165,349,120,36,25)
    text('aux-label',t('Tillsats · eleffekt','Auxiliary power'),455,332,155)
    metric('Electric power','62168','kwatt','tank',455,358,155,36,24)
    text('immersion-label',t('Elpatron · eleffekt','Immersion heater'),620,332,165)
    metric('Immersion heater','62361','kwatt','tank',620,358,165,36,24)
    text('degree-label',t('Gradminuter','Degree minutes'),900,324,205)
    metric('Degree minutes','62167','short','house',900,350,205,36,26)
    # Highlight the fan from reported compressor speed; no simulated flow.
    for element in els:
        if element['name'].startswith('fan-blade-') or element['name']=='fan-hub':
            element['background']['color']={'field':'Compressor speed','fixed':'#5B6F8C'}
    speed_override=next(o for o in overrides if o['matcher']['options']=='Compressor speed')
    speed_override['properties'] += [{'id':'color','value':{'mode':'thresholds'}},{'id':'thresholds','value':{'mode':'absolute','steps':[{'color':'#5B6F8C','value':0},{'color':'blue','value':0.000001}]}}]
    text('pump-link',t('Värmepump →','Heat pump →'),35,402,250,28,16,'Compressor speed',color='#8AB8F0')
    text('tank-link',t('Varmvatten →','Hot water →'),455,402,330,28,16,'Tank temperature',color='#8AB8F0')
    text('house-link',t('Husvärme →','Space heating →'),875,402,250,28,16,'Heating flow',color='#8AB8F0')
    text('schematic-note',t('Schematisk koppling · linjer visar inte uppmätt flöde · kontrollera insamlingsstatus ovan','Schematic connections · lines do not represent measured flow · check collection health above'),20,448,1120,24,13)
    # Element-level links enable Canvas one-click navigation.
    links_by_field={o['matcher']['options']:next(p['value'] for p in o['properties'] if p['id']=='links') for o in overrides}
    for element in els:
        field=element['config'].get('text',{}).get('field')
        if field in links_by_field:element['links']=copy.deepcopy(links_by_field[field])
    return {'id':PANEL_ID,'title':t('Systembild · värmepump, varmvatten och hus','System overview · heat pump, hot water and home'),'type':'canvas','pluginVersion':'13.3.0','description':t('Schematisk systembild, inte ett verifierat hydraulschema. Värden är senaste rapporterade API-värden och kan vara gamla vid insamlingsfel. Saknade data visas som —. Klicka på mätvärden eller detaljlänkar för rätt flik. Electric power och Immersion heater visas separat; de summeras inte.','Schematic overview, not a verified hydraulic diagram. Values are last-reported API values and may be stale on collection failure. Missing data is shown as —. Click readings or detail links to navigate. Electric power and Immersion heater are shown separately, not summed.'),'datasource':{'type':'prometheus','uid':'${datasource}'},'targets':targets,'fieldConfig':{'defaults':{'color':{'mode':'fixed','fixedColor':'#F4F5F8'},'decimals':1,'noValue':'—','mappings':[]},'overrides':overrides},'options':{'inlineEditing':False,'panZoom':True,'zoomToContent':True,'showAdvancedTypes':False,'tooltip':{'mode':'single','disableForOneClick':False},'root':{'name':'Heat pump system','type':'frame','placement':{'left':0,'top':0,'width':1160,'height':480},'background':{'color':{'fixed':'transparent'}},'elements':els}},'gridPos':{'x':0,'y':4,'w':24,'h':15}}


def v2(panel):
    queries=[]
    for q in panel['targets']:
        spec={k:v for k,v in q.items() if k not in ('datasource','refId')}
        queries.append({'kind':'PanelQuery','spec':{'refId':q['refId'],'hidden':False,'query':{'kind':'DataQuery','group':'prometheus','version':'v0','datasource':{'name':'${datasource}'},'spec':spec}}})
    return {'kind':'Panel','spec':{'id':PANEL_ID,'title':panel['title'],'description':panel['description'],'links':[],'data':{'kind':'QueryGroup','spec':{'queries':queries,'transformations':[],'queryOptions':{}}},'vizConfig':{'kind':'VizConfig','group':'canvas','version':'13.3.0','spec':{'fieldConfig':panel['fieldConfig'],'options':panel['options']}}}}


def main():
    for path in (ROOT/'dashboards').glob('ctc-ecozenith-i360*.json'):
        d=json.loads(path.read_text()); dynamic='.dynamic' in path.name
        p=canvas('.en.' in path.name,dynamic)
        if dynamic:
            d['spec']['elements']['panel-500']=v2(p)
            items=d['spec']['layout']['spec']['tabs'][0]['spec']['layout']['spec']['items']
            items[:]=[it for it in items if it['spec']['element']['name'] not in ['panel-500','panel-402','panel-403','panel-404','panel-405','panel-300','panel-205']]
            for id_,y,h in [(500,3,15),(300,18,8),(205,26,7)]:
                items.append({'kind':'GridLayoutItem','spec':{'x':0,'y':y,'width':24,'height':h,'element':{'kind':'ElementReference','name':'panel-'+str(id_)}}})
            for id_ in (402,403,404,405):d['spec']['elements'].pop('panel-'+str(id_),None)
        else:
            # Idempotent insertion before the existing timeline; retain classic rows.
            if not any(x['id']==PANEL_ID for x in d['panels']):
                timeline=next(x for x in d['panels'] if x['id']==300);y=timeline['gridPos']['y']
                for x in d['panels']:
                    if x['gridPos']['y']>=y:x['gridPos']['y']+=15
                p['gridPos']['y']=y;d['panels'].insert(d['panels'].index(timeline),p)
            else:
                i=next(i for i,x in enumerate(d['panels']) if x['id']==PANEL_ID);p['gridPos']=d['panels'][i]['gridPos'];d['panels'][i]=p
        path.write_text(json.dumps(d,ensure_ascii=False,indent=None if dynamic else 2)+'\n')
        print(path.name)

if __name__=='__main__':main()
