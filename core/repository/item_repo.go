/**
 * Copyright 2015 @ z3q.net.
 * name : goods_rep
 * author : jarryliu
 * date : -- :
 * description :
 * history :
 */
package repository

import (
	"database/sql"
	"fmt"
	"github.com/jsix/gof/db"
	"github.com/jsix/gof/db/orm"
	"go2o/core/domain/interface/enum"
	"go2o/core/domain/interface/express"
	"go2o/core/domain/interface/item"
	"go2o/core/domain/interface/pro_model"
	"go2o/core/domain/interface/product"
	"go2o/core/domain/interface/valueobject"
	itemImpl "go2o/core/domain/item"
	"go2o/core/infrastructure/format"
	"log"
)

var _ item.IGoodsItemRepo = new(goodsRepo)

type goodsRepo struct {
	db.Connector
	_orm         orm.Orm
	_skuService  item.ISkuService
	_snapService item.ISnapshotService
	catRepo      product.ICategoryRepo
	proRepo      product.IProductRepo
	itemWsRepo   item.IItemWholesaleRepo
	expressRepo  express.IExpressRepo
	valRepo      valueobject.IValueRepo
	proMRepo     promodel.IProModelRepo
}

// 商品仓储
func NewGoodsItemRepo(c db.Connector, catRepo product.ICategoryRepo,
	proRepo product.IProductRepo, proMRepo promodel.IProModelRepo,
	itemWsRepo item.IItemWholesaleRepo, expressRepo express.IExpressRepo,
	valRepo valueobject.IValueRepo) *goodsRepo {
	return &goodsRepo{
		Connector:   c,
		_orm:        c.GetOrm(),
		catRepo:     catRepo,
		proRepo:     proRepo,
		proMRepo:    proMRepo,
		itemWsRepo:  itemWsRepo,
		expressRepo: expressRepo,
		valRepo:     valRepo,
	}
}

// 获取SKU服务
func (g *goodsRepo) SkuService() item.ISkuService {
	if g._skuService == nil {
		g._skuService = itemImpl.NewSkuServiceImpl(g, g.proMRepo)
	}
	return g._skuService
}

// 获取快照服务
func (g *goodsRepo) SnapshotService() item.ISnapshotService {
	if g._snapService == nil {
		g._snapService = itemImpl.NewSnapshotServiceImpl(g)
	}
	return g._snapService
}

// 创建商品
func (g *goodsRepo) CreateItem(v *item.GoodsItem) item.IGoodsItem {
	return itemImpl.NewItem(g.proRepo, g.catRepo, nil, v, g.valRepo, g,
		g.proMRepo, g.itemWsRepo, g.expressRepo, nil)
}

// 获取商品
func (g *goodsRepo) GetItem(itemId int32) item.IGoodsItem {
	v := g.GetValueGoodsById(itemId)
	if v != nil {
		return g.CreateItem(v)
	}
	return nil
}

// 根据SKU-ID获取商品,SKU-ID为商品ID
func (g *goodsRepo) GetGoodsBySkuId(skuId int32) interface{} {
	snap := g.GetLatestSnapshot(skuId)
	if snap != nil {
		return g.GetItem(skuId)
	}
	return nil
}

// 获取商品
func (g *goodsRepo) GetValueGoods(itemId int32, skuId int32) *item.GoodsItem {
	var e *item.GoodsItem = new(item.GoodsItem)
	if g.Connector.GetOrm().GetBy(e, "product_id=? AND sku_id=?", itemId, skuId) == nil {
		return e
	}
	return nil
}

// 获取商品
func (g *goodsRepo) GetValueGoodsById(goodsId int32) *item.GoodsItem {
	var e *item.GoodsItem = new(item.GoodsItem)
	if g.Connector.GetOrm().Get(goodsId, e) == nil {
		return e
	}
	return nil
}

// 根据SKU获取商品
func (g *goodsRepo) GetValueGoodsBySku(itemId, sku int32) *item.GoodsItem {
	var e *item.GoodsItem = new(item.GoodsItem)
	if g.Connector.GetOrm().GetBy(e, "product_id=? AND sku_id=?", itemId, sku) == nil {
		return e
	}
	return nil
}

// 根据编号获取商品
func (g *goodsRepo) GetGoodsByIds(ids ...int32) ([]*valueobject.Goods, error) {
	var items []*valueobject.Goods
	err := g.Connector.GetOrm().SelectByQuery(&items,
		`SELECT * FROM item_info INNER JOIN pro_product ON item_info.product_id=pro_product.id
     WHERE item_info.id IN (`+format.IdArrJoinStr32(ids)+`)`)

	return items, err
}

// 获取会员价
func (g *goodsRepo) GetGoodsLevelPrice(goodsId int32) []*item.MemberPrice {
	var items []*item.MemberPrice
	if g.Connector.GetOrm().SelectByQuery(&items,
		`SELECT * FROM gs_member_price WHERE goods_id = ?`, goodsId) == nil {
		return items
	}
	return nil
}

// 保存会员价
func (g *goodsRepo) SaveGoodsLevelPrice(v *item.MemberPrice) (int32, error) {
	return orm.I32(orm.Save(g.GetOrm(), v, int(v.Id)))
}

// 移除会员价
func (g *goodsRepo) RemoveGoodsLevelPrice(id int32) error {
	return g.Connector.GetOrm().DeleteByPk(item.MemberPrice{}, id)
}

// 保存商品
func (g *goodsRepo) SaveValueGoods(v *item.GoodsItem) (int32, error) {
	return orm.I32(orm.Save(g.GetOrm(), v, int(v.Id)))
}

// 获取已上架的商品
func (g *goodsRepo) GetPagedOnShelvesGoods(shopId int32, catIds []int32,
	start, end int, where, orderBy string) (int, []*valueobject.Goods) {
	var sql string
	total := 0
	catIdStr := ""
	if catIds != nil && len(catIds) > 0 {
		catIdStr = fmt.Sprintf(" AND cat.id IN (%s)",
			format.IdArrJoinStr32(catIds))
	}

	if len(where) != 0 {
		where = " AND " + where
	}
	if len(orderBy) != 0 {
		orderBy += ","
	}

	list := []*valueobject.Goods{}
	g.Connector.ExecScalar(fmt.Sprintf(`SELECT COUNT(0) FROM item_info it
	  INNER JOIN pro_category cat ON it.cat_id=cat.id
		 WHERE (?<=0 OR it.shop_id =?) %s AND it.review_state=?
		  AND it.shelve_state=? %s`,
		catIdStr, where), &total, shopId, shopId, enum.ReviewPass, item.ShelvesOn)

	if total > 0 {
		sql = fmt.Sprintf(`SELECT it.* FROM item_info it INNER JOIN pro_category cat ON it.cat_id=cat.id
		 WHERE (?<=0 OR it.shop_id =?) %s AND it.review_state=? AND it.shelve_state=?
		  %s ORDER BY %s it.sort_num DESC,it.update_time DESC LIMIT ?,?`, catIdStr, where, orderBy)
		g.Connector.GetOrm().SelectByQuery(&list, sql, shopId, shopId,
			enum.ReviewPass, item.ShelvesOn, start, (end - start))
	}

	return total, list
}

// 获取指定数量已上架的商品
func (g *goodsRepo) GetOnShelvesGoods(mchId int32, start, end int, sortBy string) []*valueobject.Goods {
	e := []*valueobject.Goods{}
	sql := fmt.Sprintf(`SELECT * FROM item_info INNER JOIN pro_product ON pro_product.id = item_info.product_id
		 INNER JOIN cat_category ON pro_product.cat_id=cat_category.id
		 WHERE supplier_id=? AND pro_product.review_state=? AND pro_product.shelve_state=?
		 ORDER BY %s,update_time DESC LIMIT ?,?`,
		sortBy)

	g.Connector.GetOrm().SelectByQuery(&e, sql, mchId, enum.ReviewPass,
		item.ShelvesOn, start, (end - start))
	return e
}

// 保存快照
func (g *goodsRepo) SaveSnapshot(v *item.Snapshot) (int32, error) {
	i, _, err := g.Connector.GetOrm().Save(v.ItemId, v)
	if i == 0 {
		_, _, err = g.Connector.GetOrm().Save(nil, v)
	}
	return v.ItemId, err
}

// 获取最新的商品快照
func (g *goodsRepo) GetLatestSnapshot(itemId int32) *item.Snapshot {
	e := &item.Snapshot{}
	if g.Connector.GetOrm().Get(itemId, e) == nil {
		return e
	}
	return nil
}

// 根据指定商品快照
func (g *goodsRepo) GetSnapshots(skuIdArr []int32) []item.Snapshot {
	list := []item.Snapshot{}
	g.Connector.GetOrm().Select(&list, `item_id IN (`+
		format.IdArrJoinStr32(skuIdArr)+`)`)
	return list
}

// 获取最新的商品销售快照
func (g *goodsRepo) GetLatestSalesSnapshot(skuId int32) *item.TradeSnapshot {
	e := new(item.TradeSnapshot)
	if g.Connector.GetOrm().GetBy(e, "sku_id=? ORDER BY id DESC", skuId) == nil {
		return e
	}
	return nil
}

// 获取指定的商品销售快照
func (g *goodsRepo) GetSalesSnapshot(id int32) *item.TradeSnapshot {
	e := new(item.TradeSnapshot)
	if g.Connector.GetOrm().Get(id, e) == nil {
		return e
	}
	return nil
}

// 根据Key获取商品销售快照
func (g *goodsRepo) GetSaleSnapshotByKey(key string) *item.TradeSnapshot {
	var e *item.TradeSnapshot = new(item.TradeSnapshot)
	if g.Connector.GetOrm().GetBy(e, "key=?", key) == nil {
		return e
	}
	return nil
}

// 保存商品销售快照
func (g *goodsRepo) SaveSalesSnapshot(v *item.TradeSnapshot) (int32, error) {
	return orm.I32(orm.Save(g.Connector.GetOrm(), v, int(v.Id)))
}

// Get ItemSku
func (i *goodsRepo) GetItemSku(primary interface{}) *item.Sku {
	e := item.Sku{}
	err := i._orm.Get(primary, &e)
	if err == nil {
		return &e
	}
	if err != sql.ErrNoRows {
		log.Println("[ Orm][ Error]:", err.Error(), "; Entity:ItemSku")
	}
	return nil
}

// Select ItemSku
func (i *goodsRepo) SelectItemSku(where string, v ...interface{}) []*item.Sku {
	list := []*item.Sku{}
	err := i._orm.Select(&list, where, v...)
	if err != nil && err != sql.ErrNoRows {
		log.Println("[ Orm][ Error]:", err.Error(), "; Entity:ItemSku")
	}
	return list
}

// Save ItemSku
func (i *goodsRepo) SaveItemSku(v *item.Sku) (int, error) {
	id, err := orm.Save(i._orm, v, int(v.ID))
	if err != nil && err != sql.ErrNoRows {
		log.Println("[ Orm][ Error]:", err.Error(), "; Entity:ItemSku")
	}
	return id, err
}

// Delete ItemSku
func (i *goodsRepo) DeleteItemSku(primary interface{}) error {
	err := i._orm.DeleteByPk(item.Sku{}, primary)
	if err != nil && err != sql.ErrNoRows {
		log.Println("[ Orm][ Error]:", err.Error(), "; Entity:ItemSku")
	}
	return err
}

// Batch Delete ItemSku
func (i *goodsRepo) BatchDeleteItemSku(where string, v ...interface{}) (int64, error) {
	r, err := i._orm.Delete(item.Sku{}, where, v...)
	if err != nil && err != sql.ErrNoRows {
		log.Println("[ Orm][ Error]:", err.Error(), "; Entity:ItemSku")
	}
	return r, err
}
