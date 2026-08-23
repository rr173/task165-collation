package service
import ("context";"testing")
func TestBug05_SnapshotStoresFrozenLinks(t *testing.T){svc,_:=newTestService(t);ctx:=context.Background();p,_:=svc.CreateProject(ctx,"链接冻结","");b,_:=svc.CreateWitness(ctx,p.ID,"B","底本","",true);if _,e:=svc.ImportPassages(ctx,b.ID,"正文。");e!=nil{t.Fatal(e)};sn,e:=svc.BuildSnapshot(ctx,p.ID);if e!=nil{t.Fatal(e)};v,e:=svc.GetSnapshot(ctx,sn.ID);if e!=nil{t.Fatal(e)};if len(v.PassageHashes)==0{t.Fatal("snapshot has no frozen links")}}
