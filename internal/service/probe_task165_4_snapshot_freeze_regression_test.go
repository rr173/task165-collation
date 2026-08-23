package service
import ("context"; "testing")
func TestBug04_BuildSnapshotPersistsEvidence(t *testing.T) { svc,_:=newTestService(t);ctx:=context.Background();p,_:=svc.CreateProject(ctx,"快照冻结","");b,_:=svc.CreateWitness(ctx,p.ID,"B","底本","",true);if _,err:=svc.ImportPassages(ctx,b.ID,"冻结内容。");err!=nil{t.Fatal(err)};sn,err:=svc.BuildSnapshot(ctx,p.ID);if err!=nil{t.Fatal(err)};v,err:=svc.GetSnapshot(ctx,sn.ID);if err!=nil{t.Fatal(err)};if len(v.PassageHashes)!=1{t.Fatal("snapshot lost frozen evidence")}}
